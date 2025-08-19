package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"claude-code-companion/internal/config"
	"claude-code-companion/internal/proxy"
	"claude-code-companion/internal/utils"
	"claude-code-companion/internal/webres"
)

var (
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	port       = flag.Int("port", 0, "Override proxy server port")
	version    = flag.Bool("version", false, "Show version information")
	
	// This will be set by build process
	Version = "dev"
)


// EmbeddedAssetProvider implements webres.AssetProvider using embedded assets
type EmbeddedAssetProvider struct{}

// NewEmbeddedAssetProvider creates a new provider
func NewEmbeddedAssetProvider() *EmbeddedAssetProvider {
	return &EmbeddedAssetProvider{}
}

// GetTemplateFS returns the embedded template filesystem
func (p *EmbeddedAssetProvider) GetTemplateFS() (fs.FS, error) {
	if UseEmbedded {
		return fs.Sub(WebAssets, "web/templates")
	}
	return os.DirFS("web/templates"), nil
}

// GetStaticFS returns the embedded static filesystem  
func (p *EmbeddedAssetProvider) GetStaticFS() (fs.FS, error) {
	if UseEmbedded {
		return fs.Sub(WebAssets, "web/static")
	}
	return os.DirFS("web/static"), nil
}

// GetLocalesFS returns the embedded locales filesystem
func (p *EmbeddedAssetProvider) GetLocalesFS() (fs.FS, error) {
	if UseEmbedded {
		return fs.Sub(WebAssets, "web/locales")
	}
	return os.DirFS("web/locales"), nil
}

// LoadTemplates loads all templates from embedded filesystem
func (p *EmbeddedAssetProvider) LoadTemplates() (*template.Template, error) {
	templateFS, err := p.GetTemplateFS()
	if err != nil {
		return nil, err
	}
	
	return template.ParseFS(templateFS, "*.html")
}

// ReadLocaleFile reads a locale file from embedded filesystem
func (p *EmbeddedAssetProvider) ReadLocaleFile(filename string) ([]byte, error) {
	localesFS, err := p.GetLocalesFS()
	if err != nil {
		return nil, err
	}
	
	return fs.ReadFile(localesFS, filename)
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("Claude Code Companion %s\n", Version)
		os.Exit(0)
	}

	// Initialize embedded web assets
	webres.SetProvider(NewEmbeddedAssetProvider())

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
	proxyServer, err := proxy.NewServer(cfg, *configFile, Version)
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

	fmt.Printf("\n=== Claude Code Companion %s ===\n", Version)
	fmt.Printf("Proxy Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Admin Interface: http://%s:%d/admin/\n", cfg.Server.Host, cfg.Server.Port)
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
	
	var err error
	if proxyTimeouts.TLSHandshake, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.Proxy.TLSHandshake, "proxy.tls_handshake", 10*time.Second); err != nil {
		return err
	}
	
	if proxyTimeouts.ResponseHeader, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.Proxy.ResponseHeader, "proxy.response_header", 60*time.Second); err != nil {
		return err
	}
	
	if proxyTimeouts.IdleConnection, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.Proxy.IdleConnection, "proxy.idle_connection", 90*time.Second); err != nil {
		return err
	}
	
	// Overall request timeout is optional for proxy (streaming support)
	if proxyTimeouts.OverallRequest, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.Proxy.OverallRequest, "proxy.overall_request", 0); err != nil {
		return err
	}
	
	// Parse health check timeouts
	healthTimeouts := utils.TimeoutConfig{}
	
	if healthTimeouts.TLSHandshake, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.HealthCheck.TLSHandshake, "health_check.tls_handshake", 5*time.Second); err != nil {
		return err
	}
	
	if healthTimeouts.ResponseHeader, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.HealthCheck.ResponseHeader, "health_check.response_header", 30*time.Second); err != nil {
		return err
	}
	
	if healthTimeouts.IdleConnection, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.HealthCheck.IdleConnection, "health_check.idle_connection", 60*time.Second); err != nil {
		return err
	}
	
	if healthTimeouts.OverallRequest, err = utils.ParseTimeoutWithDefault(cfg.Timeouts.HealthCheck.OverallRequest, "health_check.overall_request", 30*time.Second); err != nil {
		return err
	}
	
	// Initialize HTTP clients
	utils.InitHTTPClientsWithTimeouts(proxyTimeouts, healthTimeouts)
	
	return nil
}