package logger

import (
	"time"
	"gorm.io/gorm/logger"
)

// GORMConfig contains configuration for GORM storage
type GORMConfig struct {
	DBPath          string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultGORMConfig returns default configuration for GORM storage
func DefaultGORMConfig(dbPath string) *GORMConfig {
	return &GORMConfig{
		DBPath:          dbPath,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		LogLevel:        logger.Silent, // 保持静默，不输出GORM日志
	}
}