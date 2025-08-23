package i18n

// Language represents a supported language
type Language string

const (
	LanguageZhCN Language = "zh-cn"
	LanguageEn   Language = "en"
	LanguageDe   Language = "de"
	LanguageEs   Language = "es"
	LanguageIt   Language = "it"
	LanguageJa   Language = "ja"
	LanguageKo   Language = "ko"
	LanguagePt   Language = "pt"
	LanguageRu   Language = "ru"
)

// Config holds i18n configuration
type Config struct {
	DefaultLanguage Language `json:"default_language" yaml:"default_language"`
	LocalesPath     string   `json:"locales_path" yaml:"locales_path"`
	Enabled         bool     `json:"enabled" yaml:"enabled"`
}

// DefaultConfig returns default i18n configuration using default values
func DefaultConfig() *Config {
	return &Config{
		DefaultLanguage: LanguageZhCN,
		LocalesPath:     "locales",
		Enabled:         true,
	}
}

// IsValidLanguage checks if the given language is supported
func IsValidLanguage(lang string) bool {
	switch Language(lang) {
	case LanguageZhCN, LanguageEn, LanguageDe, LanguageEs, LanguageIt, LanguageJa, LanguageKo, LanguagePt, LanguageRu:
		return true
	default:
		return false
	}
}