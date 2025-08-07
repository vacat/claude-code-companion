package proxy

import (
	"fmt"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/health"
	"claude-proxy/internal/logger"
	"claude-proxy/internal/validator"
	"claude-proxy/internal/web"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	validator       *validator.ResponseValidator
	healthChecker   *health.Checker
	adminServer     *web.AdminServer
	router          *gin.Engine
	configFilePath  string
}

func NewServer(cfg *config.Config, configFilePath string) (*Server, error) {
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

	// 创建管理界面服务器（如果启用）
	var adminServer *web.AdminServer
	if cfg.WebAdmin.Enabled {
		adminServer = web.NewAdminServer(cfg, endpointManager, log, configFilePath)
	}

	server := &Server{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		validator:       responseValidator,
		healthChecker:   healthChecker,
		adminServer:     adminServer,
		configFilePath:  configFilePath,
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

	// 注册管理界面路由（不需要认证）
	if s.adminServer != nil {
		s.adminServer.RegisterRoutes(s.router)
	}

	// 为 API 端点添加认证和日志中间件
	apiGroup := s.router.Group("/v1")
	apiGroup.Use(s.authMiddleware())
	apiGroup.Use(s.loggingMiddleware())
	{
		apiGroup.Any("/*path", s.handleProxy)
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.logger.Info(fmt.Sprintf("Starting proxy server on %s:%d", s.config.Server.Host, s.config.Server.Port))
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