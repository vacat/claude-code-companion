package i18n

import (
	"regexp"
	"strings"
)

// Translator handles comment-style translation processing
type Translator struct {
	// commentPattern matches pattern: >text<!-- t:translation -->
	commentPattern *regexp.Regexp
}

// NewTranslator creates a new translator
func NewTranslator() *Translator {
	// Pattern explanation:
	// data-t="([^"]+)"  - matches data-t attribute with translation
	// >([^<]+)<         - matches text content between tags
	pattern := regexp.MustCompile(`data-t="([^"]+)">([^<]+)<`)
	
	return &Translator{
		commentPattern: pattern,
	}
}

// ProcessHTML processes HTML content and replaces comment-style translations
func (t *Translator) ProcessHTML(html string, lang Language, getTranslation func(string, Language) string) string {
	if lang == LanguageZhCN {
		// For Chinese (default), return original HTML
		return html
	}
	
	// Find all comment-style translation markers
	matches := t.commentPattern.FindAllStringSubmatch(html, -1)
	result := html
	
	for _, match := range matches {
		if len(match) >= 3 {
			commentTranslation := strings.TrimSpace(match[1])  // Total Endpoints
			originalText := strings.TrimSpace(match[2])  // 端点总数
			
			// Get translation from manager (fallback to attribute translation)
			translation := getTranslation(originalText, lang)
			if translation == originalText && commentTranslation != "" {
				// If no translation found in language files, use attribute translation
				translation = commentTranslation
			}
			
			// Replace the text content while keeping the structure  
			replacement := ">" + translation + "<"
			result = strings.Replace(result, ">"+originalText+"<", replacement, 1)
		}
	}
	
	return result
}

// ExtractTranslations extracts all translatable texts from HTML
func (t *Translator) ExtractTranslations(html string) map[string]string {
	translations := make(map[string]string)
	
	matches := t.commentPattern.FindAllStringSubmatch(html, -1)
	
	for _, match := range matches {
		if len(match) >= 3 {
			originalText := strings.TrimSpace(match[1])
			commentTranslation := strings.TrimSpace(match[2])
			
			if originalText != "" && commentTranslation != "" {
				translations[originalText] = commentTranslation
			}
		}
	}
	
	return translations
}

// ValidateTranslationMarkers validates comment-style translation markers in HTML
func (t *Translator) ValidateTranslationMarkers(html string) []ValidationError {
	var errors []ValidationError
	
	matches := t.commentPattern.FindAllStringSubmatch(html, -1)
	
	for i, match := range matches {
		if len(match) < 3 {
			errors = append(errors, ValidationError{
				Index:   i,
				Message: "Invalid translation marker format",
				Pattern: match[0],
			})
			continue
		}
		
		originalText := strings.TrimSpace(match[1])
		commentTranslation := strings.TrimSpace(match[2])
		
		if originalText == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Message: "Empty original text",
				Pattern: match[0],
			})
		}
		
		if commentTranslation == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Message: "Empty translation text",
				Pattern: match[0],
			})
		}
	}
	
	return errors
}

// ValidationError represents a validation error in translation markers
type ValidationError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
	Pattern string `json:"pattern"`
}

// GenerateTranslationTemplate generates a translation template from HTML
func (t *Translator) GenerateTranslationTemplate(html string, targetLang Language) map[string]string {
	template := make(map[string]string)
	
	// Extract existing translations from comments
	extracted := t.ExtractTranslations(html)
	
	for original, translation := range extracted {
		template[original] = translation
	}
	
	return template
}

// HasTranslationMarkers checks if HTML contains any translation markers
func (t *Translator) HasTranslationMarkers(html string) bool {
	return t.commentPattern.MatchString(html)
}

// CountTranslationMarkers counts the number of translation markers in HTML
func (t *Translator) CountTranslationMarkers(html string) int {
	matches := t.commentPattern.FindAllString(html, -1)
	return len(matches)
}