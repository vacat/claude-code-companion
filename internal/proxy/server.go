package proxy

import (
	"fmt"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/health"
	"claude-proxy/internal/logger"
	"claude-proxy/internal/validator"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	validator       *validator.ResponseValidator
	healthChecker   *health.Checker
	router          *gin.Engine
}

func NewServer(cfg *config.Config) (*Server, error) {
	logConfig := logger.LogConfig{
		Level:           cfg.Logging.Level,
		LogRequestTypes: cfg.Logging.LogRequestTypes,
		LogRequestBody:  cfg.Logging.LogRequestBody,
		LogResponseBody: cfg.Logging.LogResponseBody,
		LogDirectory:    cfg.Logging.LogDirectory,
	}

	log, err := logger.NewLogger(logConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	endpointManager := endpoint.NewManager(cfg)
	responseValidator := validator.NewResponseValidator(cfg.Validation.StrictAnthropicFormat)
	healthChecker := health.NewChecker()

	server := &Server{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		validator:       responseValidator,
		healthChecker:   healthChecker,
	}
	
	// 让端点管理器使用同一个健康检查器
	endpointManager.SetHealthChecker(healthChecker)

	server.setupRoutes()
	return server, nil
}

func (s *Server) setupRoutes() {
	if s.config.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.Use(s.authMiddleware())
	s.router.Use(s.loggingMiddleware())

	v1 := s.router.Group("/v1")
	{
		v1.Any("/*path", s.handleProxy)
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Server.Port)
	s.logger.Info(fmt.Sprintf("Starting proxy server on port %d", s.config.Server.Port))
	return s.router.Run(addr)
}

func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

func (s *Server) GetEndpointManager() *endpoint.Manager {
	return s.endpointManager
}

func (s *Server) GetLogger() *logger.Logger {
	return s.logger
}

func (s *Server) GetHealthChecker() *health.Checker {
	return s.healthChecker
}