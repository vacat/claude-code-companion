package i18n

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
)

// Manager manages internationalization functionality
type Manager struct {
	config      *Config
	detector    *Detector
	translator  *Translator
	translations map[Language]map[string]string
	mu          sync.RWMutex
}

// NewManager creates a new i18n manager
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	manager := &Manager{
		config:       config,
		detector:     NewDetector(config.DefaultLanguage),
		translator:   NewTranslator(),
		translations: make(map[Language]map[string]string),
	}
	
	// Load translation files
	if err := manager.loadTranslations(); err != nil {
		return nil, fmt.Errorf("failed to load translations: %w", err)
	}
	
	return manager, nil
}

// loadTranslations loads all translation files from the locales directory
func (m *Manager) loadTranslations() error {
	if !m.config.Enabled {
		return nil
	}
	
	supportedLangs := []Language{LanguageEn, LanguageJa}
	
	for _, lang := range supportedLangs {
		filename := filepath.Join(m.config.LocalesPath, string(lang)+".json")
		translations, err := m.loadTranslationFile(filename)
		if err != nil {
			// Create empty translation map for this language
			m.translations[lang] = make(map[string]string)
			continue
		}
		
		m.translations[lang] = translations
	}
	
	return nil
}

// loadTranslationFile loads a single translation file
func (m *Manager) loadTranslationFile(filename string) (map[string]string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	return translations, nil
}

// GetDetector returns the language detector
func (m *Manager) GetDetector() *Detector {
	return m.detector
}

// GetTranslator returns the translator
func (m *Manager) GetTranslator() *Translator {
	return m.translator
}

// IsEnabled returns whether i18n is enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetDefaultLanguage returns the default language
func (m *Manager) GetDefaultLanguage() Language {
	return m.config.DefaultLanguage
}

// GetTranslation gets a translation for the given text and language
func (m *Manager) GetTranslation(text string, lang Language) string {
	if !m.config.Enabled {
		return text
	}
	
	// If it's the default language, return original text
	if lang == m.config.DefaultLanguage {
		return text
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if langTranslations, exists := m.translations[lang]; exists {
		if translation, found := langTranslations[text]; found {
			return translation
		}
	}
	
	// Fallback to original text
	return text
}

// ReloadTranslations reloads all translation files
func (m *Manager) ReloadTranslations() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Clear existing translations
	m.translations = make(map[Language]map[string]string)
	
	// Reload translations
	return m.loadTranslations()
}

// AddTranslation adds a new translation dynamically
func (m *Manager) AddTranslation(lang Language, original, translation string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.translations[lang] == nil {
		m.translations[lang] = make(map[string]string)
	}
	
	m.translations[lang][original] = translation
}

// GetAvailableLanguages returns all available languages
func (m *Manager) GetAvailableLanguages() []Language {
	return []Language{LanguageZhCN, LanguageEn, LanguageJa}
}

// GetLanguageInfo returns display information for a language
func (m *Manager) GetLanguageInfo(lang Language) map[string]string {
	switch lang {
	case LanguageZhCN:
		return map[string]string{"flag": "CN", "name": "中文"}
	case LanguageEn:
		return map[string]string{"flag": "US", "name": "English"}
	case LanguageJa:
		return map[string]string{"flag": "JP", "name": "日本語"}
	default:
		return map[string]string{"flag": "??", "name": string(lang)}
	}
}

// GetAllTranslations returns all translations for debugging
func (m *Manager) GetAllTranslations() map[Language]map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make(map[Language]map[string]string)
	for lang, translations := range m.translations {
		result[lang] = make(map[string]string)
		for key, value := range translations {
			result[lang][key] = value
		}
	}
	
	return result
}