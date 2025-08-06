package logger

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type RequestLog struct {
	Timestamp       time.Time         `json:"timestamp"`
	RequestID       string            `json:"request_id"`
	Endpoint        string            `json:"endpoint"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	StatusCode      int               `json:"status_code"`
	DurationMs      int64             `json:"duration_ms"`
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     string            `json:"request_body"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body"`
	Error           string            `json:"error,omitempty"`
	RequestBodySize int               `json:"request_body_size"`
	ResponseBodySize int              `json:"response_body_size"`
	IsStreaming     bool              `json:"is_streaming"`
}

type Logger struct {
	logger       *logrus.Logger
	storage      *Storage
	config       LogConfig
}

type LogConfig struct {
	Level              string
	LogFailedRequests  bool
	LogRequestBody     bool
	LogResponseBody    bool
	PersistToDisk      bool
	LogDirectory       string
}

func NewLogger(config LogConfig) (*Logger, error) {
	logger := logrus.New()
	
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	var storage *Storage
	if config.PersistToDisk {
		storage, err = NewStorage(config.LogDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize log storage: %v", err)
		}
	}

	return &Logger{
		logger:  logger,
		storage: storage,
		config:  config,
	}, nil
}

func (l *Logger) LogRequest(log *RequestLog) {
	// 总是记录到存储，方便Web界面查看
	if l.storage != nil {
		l.storage.SaveLog(log)
	}

	// 根据配置决定是否输出到控制台
	shouldLog := true
	if l.config.LogFailedRequests && log.StatusCode < 400 {
		shouldLog = false
	}

	if shouldLog {
		fields := logrus.Fields{
			"request_id":   log.RequestID,
			"endpoint":     log.Endpoint,
			"method":       log.Method,
			"path":         log.Path,
			"status_code":  log.StatusCode,
			"duration_ms":  log.DurationMs,
		}

		if log.Error != "" {
			fields["error"] = log.Error
		}

		if l.config.LogRequestBody && log.RequestBody != "" {
			fields["request_body"] = log.RequestBody
		}

		if l.config.LogResponseBody && log.ResponseBody != "" {
			fields["response_body"] = log.ResponseBody
		}

		if log.StatusCode >= 400 {
			l.logger.WithFields(fields).Error("Request failed")
		} else {
			l.logger.WithFields(fields).Info("Request completed")
		}
	}
}

func (l *Logger) Info(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		l.logger.WithFields(fields[0]).Info(msg)
	} else {
		l.logger.Info(msg)
	}
}

func (l *Logger) Error(msg string, err error, fields ...logrus.Fields) {
	baseFields := logrus.Fields{}
	if err != nil {
		baseFields["error"] = err.Error()
	}
	
	if len(fields) > 0 {
		for k, v := range fields[0] {
			baseFields[k] = v
		}
	}
	
	l.logger.WithFields(baseFields).Error(msg)
}

func (l *Logger) Debug(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		l.logger.WithFields(fields[0]).Debug(msg)
	} else {
		l.logger.Debug(msg)
	}
}

func (l *Logger) GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error) {
	if l.storage == nil {
		return []*RequestLog{}, 0, nil
	}
	return l.storage.GetLogs(limit, offset, failedOnly)
}

func (l *Logger) GetAllLogsByRequestID(requestID string) ([]*RequestLog, error) {
	if l.storage == nil {
		return []*RequestLog{}, nil
	}
	return l.storage.GetAllLogsByRequestID(requestID)
}

func headersToMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func (l *Logger) CreateRequestLog(requestID, endpoint, method, path string) *RequestLog {
	return &RequestLog{
		Timestamp: time.Now(),
		RequestID: requestID,
		Endpoint:  endpoint,
		Method:    method,
		Path:      path,
	}
}

func (l *Logger) UpdateRequestLog(log *RequestLog, req *http.Request, resp *http.Response, body []byte, duration time.Duration, err error) {
	log.DurationMs = duration.Nanoseconds() / 1000000
	
	if req != nil {
		log.RequestHeaders = headersToMap(req.Header)
		log.IsStreaming = req.Header.Get("Accept") == "text/event-stream" || 
			req.Header.Get("Accept") == "application/json, text/event-stream"
	}
	
	if resp != nil {
		log.StatusCode = resp.StatusCode
		log.ResponseHeaders = headersToMap(resp.Header)
		
		// 检查响应是否为流式
		if resp.Header.Get("Content-Type") != "" {
			contentType := resp.Header.Get("Content-Type")
			if strings.Contains(contentType, "text/event-stream") {
				log.IsStreaming = true
			}
		}
	}
	
	log.ResponseBodySize = len(body)
	if l.config.LogResponseBody && len(body) > 0 {
		log.ResponseBody = string(body)
	}
	
	if err != nil {
		log.Error = err.Error()
	}
}