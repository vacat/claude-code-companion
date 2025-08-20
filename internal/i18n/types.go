package i18n

import (
	"claude-code-companion/internal/config"
)

// Language represents a supported language
type Language string

const (
	LanguageZhCN Language = "zh-cn"
	LanguageEn   Language = "en"
)

// Config holds i18n configuration
type Config struct {
	DefaultLanguage Language `json:"default_language" yaml:"default_language"`
	LocalesPath     string   `json:"locales_path" yaml:"locales_path"`
	Enabled         bool     `json:"enabled" yaml:"enabled"`
}

// DefaultConfig returns default i18n configuration using unified defaults
func DefaultConfig() *Config {
	return &Config{
		DefaultLanguage: Language(config.Default.I18n.DefaultLanguage),
		LocalesPath:     config.Default.I18n.LocalesPath,
		Enabled:         config.Default.I18n.Enabled,
	}
}

// IsValidLanguage checks if the given language is supported
func IsValidLanguage(lang string) bool {
	switch Language(lang) {
	case LanguageZhCN, LanguageEn:
		return true
	default:
		return false
	}
}