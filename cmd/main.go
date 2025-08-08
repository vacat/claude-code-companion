package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"claude-proxy/internal/config"
	"claude-proxy/internal/proxy"
	"claude-proxy/internal/utils"
)

var (
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	port       = flag.Int("port", 0, "Override proxy server port")
	version    = flag.Bool("version", false, "Show version information")
)

const Version = "1.0.0"

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("Claude Proxy Server v%s\n", Version)
		os.Exit(0)
	}

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Initialize HTTP clients with configured timeouts
	if err := initHTTPClientsFromConfig(cfg); err != nil {
		log.Fatalf("Failed to initialize HTTP clients: %v", err)
	}
	proxyServer, err := proxy.NewServer(cfg, *configFile)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	go func() {
		log.Printf("Starting proxy server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := proxyServer.Start(); err != nil {
			log.Fatalf("Proxy server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("\n=== Claude API Proxy Server v%s ===\n", Version)
	fmt.Printf("Proxy Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	if cfg.WebAdmin.Enabled {
		fmt.Printf("Admin Interface: http://%s:%d/admin/\n", cfg.Server.Host, cfg.Server.Port)
	}
	fmt.Printf("Configuration File: %s\n", *configFile)
	fmt.Printf("\nPress Ctrl+C to stop the server...\n\n")

	<-quit
	fmt.Println("\nShutting down servers...")
	
	// Graceful shutdown: close logger and database connections
	if logger := proxyServer.GetLogger(); logger != nil {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		} else {
			log.Println("Logger closed successfully")
		}
	}
}

// initHTTPClientsFromConfig initializes HTTP clients with timeout configurations
func initHTTPClientsFromConfig(cfg *config.Config) error {
	// Parse proxy timeouts
	proxyTimeouts := utils.TimeoutConfig{}
	
	if cfg.Timeouts.Proxy.TLSHandshake != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.Proxy.TLSHandshake); err != nil {
			return fmt.Errorf("invalid proxy.tls_handshake timeout: %v", err)
		} else {
			proxyTimeouts.TLSHandshake = d
		}
	} else {
		proxyTimeouts.TLSHandshake = 10 * time.Second
	}
	
	if cfg.Timeouts.Proxy.ResponseHeader != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.Proxy.ResponseHeader); err != nil {
			return fmt.Errorf("invalid proxy.response_header timeout: %v", err)
		} else {
			proxyTimeouts.ResponseHeader = d
		}
	} else {
		proxyTimeouts.ResponseHeader = 60 * time.Second
	}
	
	if cfg.Timeouts.Proxy.IdleConnection != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.Proxy.IdleConnection); err != nil {
			return fmt.Errorf("invalid proxy.idle_connection timeout: %v", err)
		} else {
			proxyTimeouts.IdleConnection = d
		}
	} else {
		proxyTimeouts.IdleConnection = 90 * time.Second
	}
	
	// Overall request timeout is optional for proxy (streaming support)
	if cfg.Timeouts.Proxy.OverallRequest != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.Proxy.OverallRequest); err != nil {
			return fmt.Errorf("invalid proxy.overall_request timeout: %v", err)
		} else {
			proxyTimeouts.OverallRequest = d
		}
	}
	
	// Parse health check timeouts
	healthTimeouts := utils.TimeoutConfig{}
	
	if cfg.Timeouts.HealthCheck.TLSHandshake != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.HealthCheck.TLSHandshake); err != nil {
			return fmt.Errorf("invalid health_check.tls_handshake timeout: %v", err)
		} else {
			healthTimeouts.TLSHandshake = d
		}
	} else {
		healthTimeouts.TLSHandshake = 5 * time.Second
	}
	
	if cfg.Timeouts.HealthCheck.ResponseHeader != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.HealthCheck.ResponseHeader); err != nil {
			return fmt.Errorf("invalid health_check.response_header timeout: %v", err)
		} else {
			healthTimeouts.ResponseHeader = d
		}
	} else {
		healthTimeouts.ResponseHeader = 30 * time.Second
	}
	
	if cfg.Timeouts.HealthCheck.IdleConnection != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.HealthCheck.IdleConnection); err != nil {
			return fmt.Errorf("invalid health_check.idle_connection timeout: %v", err)
		} else {
			healthTimeouts.IdleConnection = d
		}
	} else {
		healthTimeouts.IdleConnection = 60 * time.Second
	}
	
	if cfg.Timeouts.HealthCheck.OverallRequest != "" {
		if d, err := time.ParseDuration(cfg.Timeouts.HealthCheck.OverallRequest); err != nil {
			return fmt.Errorf("invalid health_check.overall_request timeout: %v", err)
		} else {
			healthTimeouts.OverallRequest = d
		}
	} else {
		healthTimeouts.OverallRequest = 30 * time.Second
	}
	
	// Initialize HTTP clients
	utils.InitHTTPClientsWithTimeouts(proxyTimeouts, healthTimeouts)
	
	return nil
}