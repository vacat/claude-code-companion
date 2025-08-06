package logger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Storage struct {
	logDir string
	mutex  sync.RWMutex
	logs   []*RequestLog
}

func NewStorage(logDir string) (*Storage, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	storage := &Storage{
		logDir: logDir,
		logs:   make([]*RequestLog, 0),
	}

	if err := storage.loadExistingLogs(); err != nil {
		return nil, fmt.Errorf("failed to load existing logs: %v", err)
	}

	return storage, nil
}

func (s *Storage) SaveLog(log *RequestLog) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.logs = append(s.logs, log)

	logFile := filepath.Join(s.logDir, s.getLogFileName(log.Timestamp))
	
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	logData, err := json.Marshal(log)
	if err != nil {
		return
	}

	file.Write(logData)
	file.Write([]byte("\n"))
}

func (s *Storage) GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	filteredLogs := make([]*RequestLog, 0)
	for _, log := range s.logs {
		if !failedOnly || log.StatusCode >= 400 || log.Error != "" {
			filteredLogs = append(filteredLogs, log)
		}
	}

	sort.Slice(filteredLogs, func(i, j int) bool {
		return filteredLogs[i].Timestamp.After(filteredLogs[j].Timestamp)
	})

	total := len(filteredLogs)

	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	result := make([]*RequestLog, 0)
	if start < end {
		result = filteredLogs[start:end]
	}

	return result, total, nil
}

func (s *Storage) GetAllLogsByRequestID(requestID string) ([]*RequestLog, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var matchingLogs []*RequestLog
	for _, log := range s.logs {
		if log.RequestID == requestID {
			matchingLogs = append(matchingLogs, log)
		}
	}

	// 按时间戳排序，确保按尝试顺序显示
	sort.Slice(matchingLogs, func(i, j int) bool {
		return matchingLogs[i].Timestamp.Before(matchingLogs[j].Timestamp)
	})

	return matchingLogs, nil
}

func (s *Storage) loadExistingLogs() error {
	files, err := ioutil.ReadDir(s.logDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".log") {
			continue
		}

		filePath := filepath.Join(s.logDir, file.Name())
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var log RequestLog
			if err := json.Unmarshal([]byte(line), &log); err != nil {
				continue
			}

			s.logs = append(s.logs, &log)
		}
	}

	return nil
}

func (s *Storage) getLogFileName(timestamp time.Time) string {
	return fmt.Sprintf("requests_%s.log", timestamp.Format("2006-01-02"))
}