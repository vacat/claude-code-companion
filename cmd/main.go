package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"claude-proxy/internal/config"
	"claude-proxy/internal/proxy"
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
	proxyServer, err := proxy.NewServer(cfg, *configFile)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	go func() {
		log.Printf("Starting proxy server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		log.Printf("Authorization token: %s", cfg.Server.AuthToken)
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
	fmt.Printf("Authorization Token: %s\n", cfg.Server.AuthToken)
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