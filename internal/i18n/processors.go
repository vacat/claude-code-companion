package i18n

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context represents the context for translation processing
type Context struct {
	GinContext *gin.Context
	Language   Language
	RequestID  string
	UserData   map[string]interface{}
}

// ValidationError represents validation errors in translation processing
type ProcessorValidationError struct {
	Index       int    `json:"index"`
	Message     string `json:"message"`
	Pattern     string `json:"pattern"`
	ProcessorID string `json:"processor_id"`
}

// TranslationProcessor defines the interface for translation processors
type TranslationProcessor interface {
	// GetID returns processor unique identifier
	GetID() string

	// Process processes content and replaces translation markers
	Process(content string, lang Language, ctx Context) (string, error)

	// Extract extracts translation key-value pairs from content
	Extract(content string) (map[string]string, error)

	// Validate validates translation markers in content
	Validate(content string) []ProcessorValidationError

	// GetPriority returns processing priority (lower = higher priority)
	GetPriority() int
}

// ProcessorChain manages multiple translation processors
type ProcessorChain struct {
	processors []TranslationProcessor
	manager    *Manager
}

// NewProcessorChain creates a new processor chain
func NewProcessorChain(manager *Manager) *ProcessorChain {
	chain := &ProcessorChain{
		processors: make([]TranslationProcessor, 0),
		manager:    manager,
	}

	// Register default processors in priority order
	chain.RegisterProcessor(NewHTMLTagProcessor(manager))
	chain.RegisterProcessor(NewHTMLTextProcessor(manager))
	chain.RegisterProcessor(NewHTMLAttrProcessor(manager))

	return chain
}

// RegisterProcessor adds a processor to the chain
func (pc *ProcessorChain) RegisterProcessor(processor TranslationProcessor) {
	pc.processors = append(pc.processors, processor)

	// Sort by priority
	for i := len(pc.processors) - 1; i > 0; i-- {
		if pc.processors[i].GetPriority() < pc.processors[i-1].GetPriority() {
			pc.processors[i], pc.processors[i-1] = pc.processors[i-1], pc.processors[i]
		}
	}
}

// Process processes content through all processors
func (pc *ProcessorChain) Process(content string, lang Language, ctx Context) (string, error) {
	result := content
	var err error

	for _, processor := range pc.processors {
		result, err = processor.Process(result, lang, ctx)
		if err != nil {
			return result, fmt.Errorf("processor %s failed: %w", processor.GetID(), err)
		}
	}

	return result, nil
}

// ExtractAll extracts translations from all processors
func (pc *ProcessorChain) ExtractAll(content string) (map[string]string, error) {
	result := make(map[string]string)

	for _, processor := range pc.processors {
		translations, err := processor.Extract(content)
		if err != nil {
			return nil, fmt.Errorf("processor %s extraction failed: %w", processor.GetID(), err)
		}

		// Merge translations
		for key, value := range translations {
			result[key] = value
		}
	}

	return result, nil
}

// ValidateAll validates content with all processors
func (pc *ProcessorChain) ValidateAll(content string) []ProcessorValidationError {
	var allErrors []ProcessorValidationError

	for _, processor := range pc.processors {
		errors := processor.Validate(content)
		allErrors = append(allErrors, errors...)
	}

	return allErrors
}

// HTMLTagProcessor handles HTML tag content translation (existing data-t functionality)
type HTMLTagProcessor struct {
	id      string
	manager *Manager
	pattern *regexp.Regexp
}

// NewHTMLTagProcessor creates a new HTML tag processor
func NewHTMLTagProcessor(manager *Manager) *HTMLTagProcessor {
	return &HTMLTagProcessor{
		id:      "html-tag",
		manager: manager,
		pattern: regexp.MustCompile(`(<[^>]*data-t="([^"]+)"[^>]*>)([^<]*)(</[^>]+>)`),
	}
}

func (p *HTMLTagProcessor) GetID() string    { return p.id }
func (p *HTMLTagProcessor) GetPriority() int { return 10 }

func (p *HTMLTagProcessor) Process(content string, lang Language, ctx Context) (string, error) {
	if lang == LanguageZhCN {
		return content, nil
	}

	result := content
	matches := p.pattern.FindAllStringSubmatch(result, -1)

	for _, match := range matches {
		if len(match) >= 5 {
			fullMatch := match[0]
			openTag := match[1]
			translationKey := match[2]
			originalContent := match[3]
			closeTag := match[4]

			translation := p.manager.GetTranslation(translationKey, lang)
			if translation == translationKey {
				translation = originalContent
			}

			if strings.TrimSpace(originalContent) != "" {
				replacement := openTag + translation + closeTag
				result = strings.Replace(result, fullMatch, replacement, 1)
			}
		}
	}

	return result, nil
}

func (p *HTMLTagProcessor) Extract(content string) (map[string]string, error) {
	result := make(map[string]string)
	matches := p.pattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 5 {
			key := strings.TrimSpace(match[2])
			value := strings.TrimSpace(match[3])

			if key != "" && value != "" {
				result[key] = value
			}
		}
	}

	return result, nil
}

func (p *HTMLTagProcessor) Validate(content string) []ProcessorValidationError {
	matches := p.pattern.FindAllStringSubmatch(content, -1)
	return validateTranslationMatches(matches, 2, p.id, "Invalid data-t tag format", "Empty translation key in data-t")
}

// HTMLTextProcessor handles HTML pure text translation using <!--T:key-->text<!--/T--> syntax
type HTMLTextProcessor struct {
	id      string
	manager *Manager
	pattern *regexp.Regexp
}

// NewHTMLTextProcessor creates a new HTML text processor
func NewHTMLTextProcessor(manager *Manager) *HTMLTextProcessor {
	return &HTMLTextProcessor{
		id:      "html-text",
		manager: manager,
		pattern: regexp.MustCompile(`<!--T:([^>]+)-->([^<]+)<!--/T-->`),
	}
}

func (p *HTMLTextProcessor) GetID() string    { return p.id }
func (p *HTMLTextProcessor) GetPriority() int { return 20 }

func (p *HTMLTextProcessor) Process(content string, lang Language, ctx Context) (string, error) {
	if lang == LanguageZhCN {
		return content, nil
	}

	result := content
	matches := p.pattern.FindAllStringSubmatch(result, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			fullMatch := match[0]
			translationKey := match[1]
			originalText := match[2]

			translation := p.manager.GetTranslation(translationKey, lang)
			if translation == translationKey {
				translation = originalText
			}

			result = strings.Replace(result, fullMatch, translation, 1)
		}
	}

	return result, nil
}

func (p *HTMLTextProcessor) Extract(content string) (map[string]string, error) {
	result := make(map[string]string)
	matches := p.pattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])

			if key != "" && value != "" {
				result[key] = value
			}
		}
	}

	return result, nil
}

func (p *HTMLTextProcessor) Validate(content string) []ProcessorValidationError {
	matches := p.pattern.FindAllStringSubmatch(content, -1)
	return validateTranslationMatches(matches, 1, p.id, "Invalid HTML text translation format", "Empty translation key in HTML text marker")
}

func validateTranslationMatches(matches [][]string, keyIndex int, processorID, invalidMsg, emptyKeyMsg string) []ProcessorValidationError {
	var errors []ProcessorValidationError
	for i, match := range matches {
		if len(match) <= keyIndex {
			errors = append(errors, ProcessorValidationError{
				Index:       i,
				Message:     invalidMsg,
				Pattern:     match[0],
				ProcessorID: processorID,
			})
			continue
		}

		if strings.TrimSpace(match[keyIndex]) == "" {
			errors = append(errors, ProcessorValidationError{
				Index:       i,
				Message:     emptyKeyMsg,
				Pattern:     match[0],
				ProcessorID: processorID,
			})
		}
	}
	return errors
}

// HTMLAttrProcessor handles HTML attribute translation using data-t-attribute syntax
type HTMLAttrProcessor struct {
	id      string
	manager *Manager
	pattern *regexp.Regexp
}

// NewHTMLAttrProcessor creates a new HTML attribute processor
func NewHTMLAttrProcessor(manager *Manager) *HTMLAttrProcessor {
	return &HTMLAttrProcessor{
		id:      "html-attr",
		manager: manager,
		// Simplified pattern: match data-t-attribute="key" and the corresponding attribute="value"
		pattern: regexp.MustCompile(`data-t-(\w+)="([^"]+)"`),
	}
}

func (p *HTMLAttrProcessor) GetID() string    { return p.id }
func (p *HTMLAttrProcessor) GetPriority() int { return 15 }

func (p *HTMLAttrProcessor) Process(content string, lang Language, ctx Context) (string, error) {
	if lang == LanguageZhCN {
		return content, nil
	}

	result := content

	// Find all data-t-attribute markers
	matches := p.pattern.FindAllStringSubmatch(result, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			attrName := match[1]       // attribute name (e.g., "placeholder")
			translationKey := match[2] // translation key

			// Look for the corresponding attribute in the same element
			// Pattern to find the actual attribute value
			attrPattern := regexp.MustCompile(fmt.Sprintf(`(%s=")([^"]*)(")`, regexp.QuoteMeta(attrName)))
			attrMatches := attrPattern.FindAllStringSubmatch(result, -1)

			for _, attrMatch := range attrMatches {
				if len(attrMatch) >= 4 {
					fullAttrMatch := attrMatch[0]
					beforeValue := attrMatch[1]
					originalValue := attrMatch[2]
					afterValue := attrMatch[3]

					translation := p.manager.GetTranslation(translationKey, lang)
					if translation == translationKey {
						translation = originalValue
					}

					replacement := beforeValue + translation + afterValue
					result = strings.Replace(result, fullAttrMatch, replacement, 1)
				}
			}
		}
	}

	return result, nil
}

func (p *HTMLAttrProcessor) Extract(content string) (map[string]string, error) {
	result := make(map[string]string)
	matches := p.pattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			attrName := strings.TrimSpace(match[1])
			translationKey := strings.TrimSpace(match[2])

			if attrName != "" && translationKey != "" {
				// Try to find the actual attribute value for this translation key
				attrPattern := regexp.MustCompile(fmt.Sprintf(`%s="([^"]*)"`, regexp.QuoteMeta(attrName)))
				attrMatches := attrPattern.FindAllStringSubmatch(content, -1)

				for _, attrMatch := range attrMatches {
					if len(attrMatch) >= 2 {
						value := strings.TrimSpace(attrMatch[1])
						if value != "" {
							result[translationKey] = value
							break // Take the first match
						}
					}
				}
			}
		}
	}

	return result, nil
}

func (p *HTMLAttrProcessor) Validate(content string) []ProcessorValidationError {
	var errors []ProcessorValidationError
	matches := p.pattern.FindAllStringSubmatch(content, -1)

	for i, match := range matches {
		if len(match) < 3 {
			errors = append(errors, ProcessorValidationError{
				Index:       i,
				Message:     "Invalid HTML attribute translation format",
				Pattern:     match[0],
				ProcessorID: p.id,
			})
			continue
		}

		if strings.TrimSpace(match[1]) == "" {
			errors = append(errors, ProcessorValidationError{
				Index:       i,
				Message:     "Empty attribute name in HTML attribute marker",
				Pattern:     match[0],
				ProcessorID: p.id,
			})
		}

		if strings.TrimSpace(match[2]) == "" {
			errors = append(errors, ProcessorValidationError{
				Index:       i,
				Message:     "Empty translation key in HTML attribute marker",
				Pattern:     match[0],
				ProcessorID: p.id,
			})
		}
	}

	return errors
}
