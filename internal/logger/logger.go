package logger

import (
	"encoding/json"
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
	Model           string            `json:"model,omitempty"`
}

type Logger struct {
	logger       *logrus.Logger
	storage      *Storage
	config       LogConfig
}

type LogConfig struct {
	Level           string
	LogRequestTypes string
	LogRequestBody  string
	LogResponseBody string
	LogDirectory    string
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

	storage, err := NewStorage(config.LogDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize log storage: %v", err)
	}

	return &Logger{
		logger:  logger,
		storage: storage,
		config:  config,
	}, nil
}

func (l *Logger) LogRequest(log *RequestLog) {
	// 总是记录到存储，方便Web界面查看
	l.storage.SaveLog(log)

	// 根据配置决定是否输出到控制台
	shouldLog := l.shouldLogRequest(log.StatusCode)

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

		if log.Model != "" {
			fields["model"] = log.Model
		}

		// Note: Request and response bodies are not logged to console
		// They are available in the web admin interface

		if log.StatusCode >= 400 {
			l.logger.WithFields(fields).Error("Request failed")
		} else {
			l.logger.WithFields(fields).Info("Request completed")
		}
	}
}

// shouldLogRequest determines if a request should be logged to console based on configuration
func (l *Logger) shouldLogRequest(statusCode int) bool {
	switch l.config.LogRequestTypes {
	case "failed":
		return statusCode >= 400
	case "success":
		return statusCode < 400
	case "all":
		return true
	default:
		return true
	}
}

// truncateBody truncates body content to specified length
func (l *Logger) truncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "... [truncated]"
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
	if l.config.LogResponseBody != "none" && len(body) > 0 {
		if l.config.LogResponseBody == "truncated" {
			log.ResponseBody = l.truncateBody(string(body), 1024)
		} else {
			log.ResponseBody = string(body)
		}
	}
	
	if err != nil {
		log.Error = err.Error()
	}
}

// ExtractModelFromRequestBody extracts the model name from request body JSON
func ExtractModelFromRequestBody(body string) string {
	if body == "" {
		return ""
	}
	
	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(body), &requestData); err != nil {
		return ""
	}
	
	if model, ok := requestData["model"].(string); ok {
		return model
	}
	
	return ""
}