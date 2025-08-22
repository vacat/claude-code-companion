package i18n

import (
	"context"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
)

// GlobalManager holds the global translation manager instance
var (
	globalManager *Manager
	globalMutex   sync.RWMutex
)

// SetGlobalManager sets the global translation manager
func SetGlobalManager(manager *Manager) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	globalManager = manager
}

// GetGlobalManager returns the global translation manager
func GetGlobalManager() *Manager {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return globalManager
}

// getCurrentLanguage gets current language from context or default
func getCurrentLanguage() Language {
	// Try to get from goroutine context
	if manager := GetGlobalManager(); manager != nil {
		return manager.GetDefaultLanguage()
	}
	return LanguageZhCN
}

// T provides basic translation functionality
// Usage: T("welcome_message", "欢迎使用")
func T(key, fallback string) string {
	manager := GetGlobalManager()
	if manager == nil {
		return fallback
	}
	
	lang := getCurrentLanguage()
	translation := manager.GetTranslation(key, lang)
	
	// If no translation found, return fallback
	if translation == key {
		return fallback
	}
	
	return translation
}

// Tf provides formatted translation functionality with parameters
// Usage: Tf("welcome_user", "欢迎 %s", username)
func Tf(key, fallback string, args ...interface{}) string {
	translation := T(key, fallback)
	if len(args) > 0 {
		return fmt.Sprintf(translation, args...)
	}
	return translation
}

// TCtx provides context-aware translation functionality
// Usage: TCtx(c, "error_message", "发生错误")
func TCtx(c *gin.Context, key, fallback string) string {
	if c == nil {
		return T(key, fallback)
	}
	
	manager := GetGlobalManager()
	if manager == nil {
		return fallback
	}
	
	lang := GetLanguageFromContext(c)
	translation := manager.GetTranslation(key, lang)
	
	// If no translation found, return fallback
	if translation == key {
		return fallback
	}
	
	return translation
}

// TCtxf provides context-aware formatted translation functionality
// Usage: TCtxf(c, "user_login", "用户 %s 已登录", username)
func TCtxf(c *gin.Context, key, fallback string, args ...interface{}) string {
	translation := TCtx(c, key, fallback)
	if len(args) > 0 {
		return fmt.Sprintf(translation, args...)
	}
	return translation
}

// TPlural provides plural form translation functionality
// Usage: TPlural("item_count", "1 个项目", "%d 个项目", count)
func TPlural(key, singular, plural string, count int) string {
	var template string
	if count == 1 {
		template = T(key+"_singular", singular)
	} else {
		template = T(key+"_plural", plural)
	}
	
	return fmt.Sprintf(template, count)
}

// TCtxPlural provides context-aware plural form translation
// Usage: TCtxPlural(c, "item_count", "1 个项目", "%d 个项目", count)
func TCtxPlural(c *gin.Context, key, singular, plural string, count int) string {
	var template string
	if count == 1 {
		template = TCtx(c, key+"_singular", singular)
	} else {
		template = TCtx(c, key+"_plural", plural)
	}
	
	return fmt.Sprintf(template, count)
}

// TWithLang provides translation with explicit language specification
// Usage: TWithLang("welcome", "欢迎", LanguageEn)
func TWithLang(key, fallback string, lang Language) string {
	manager := GetGlobalManager()
	if manager == nil {
		return fallback
	}
	
	translation := manager.GetTranslation(key, lang)
	if translation == key {
		return fallback
	}
	
	return translation
}

// TfWithLang provides formatted translation with explicit language
// Usage: TfWithLang("welcome_user", "欢迎 %s", LanguageEn, username)
func TfWithLang(key, fallback string, lang Language, args ...interface{}) string {
	translation := TWithLang(key, fallback, lang)
	if len(args) > 0 {
		return fmt.Sprintf(translation, args...)
	}
	return translation
}

// Helper functions for language context management

// WithLanguageContext creates a new context with language information
func WithLanguageContext(parent context.Context, lang Language) context.Context {
	return context.WithValue(parent, "language", lang)
}

// LanguageFromContext extracts language from context
func LanguageFromContext(ctx context.Context) Language {
	if lang, ok := ctx.Value("language").(Language); ok {
		return lang
	}
	return LanguageZhCN
}

// Middleware function to inject language into Gin context
func LanguageMiddleware(manager *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager == nil || !manager.IsEnabled() {
			SetLanguageToContext(c, LanguageZhCN)
			c.Next()
			return
		}
		
		detector := manager.GetDetector()
		lang := detector.DetectLanguage(c)
		SetLanguageToContext(c, lang)
		
		c.Next()
	}
}

// Translation key constants for common use cases
const (
	KeyWelcome           = "welcome"
	KeyError             = "error"
	KeySuccess           = "success"
	KeyWarning           = "warning"
	KeyInfo              = "info"
	KeyConfirm           = "confirm"
	KeyCancel            = "cancel"
	KeySave              = "save"
	KeyDelete            = "delete"
	KeyEdit              = "edit"
	KeyAdd               = "add"
	KeyLoading           = "loading"
	KeyConnecting        = "connecting"
	KeyConnected         = "connected"
	KeyDisconnected      = "disconnected"
	KeyServerError       = "server_error"
	KeyNotFound          = "not_found"
	KeyUnauthorized      = "unauthorized"
	KeyForbidden         = "forbidden"
	KeyValidationError   = "validation_error"
	KeyNetworkError      = "network_error"
)

// Convenience functions for common translation keys
func TWelcome(fallback string) string           { return T(KeyWelcome, fallback) }
func TError(fallback string) string             { return T(KeyError, fallback) }
func TSuccess(fallback string) string           { return T(KeySuccess, fallback) }
func TWarning(fallback string) string           { return T(KeyWarning, fallback) }
func TInfo(fallback string) string              { return T(KeyInfo, fallback) }
func TConfirm(fallback string) string           { return T(KeyConfirm, fallback) }
func TCancel(fallback string) string            { return T(KeyCancel, fallback) }
func TSave(fallback string) string              { return T(KeySave, fallback) }
func TDelete(fallback string) string            { return T(KeyDelete, fallback) }
func TEdit(fallback string) string              { return T(KeyEdit, fallback) }
func TAdd(fallback string) string               { return T(KeyAdd, fallback) }
func TLoading(fallback string) string           { return T(KeyLoading, fallback) }
func TConnecting(fallback string) string        { return T(KeyConnecting, fallback) }
func TConnected(fallback string) string         { return T(KeyConnected, fallback) }
func TDisconnected(fallback string) string      { return T(KeyDisconnected, fallback) }
func TServerError(fallback string) string       { return T(KeyServerError, fallback) }
func TNotFound(fallback string) string          { return T(KeyNotFound, fallback) }
func TUnauthorized(fallback string) string      { return T(KeyUnauthorized, fallback) }
func TForbidden(fallback string) string         { return T(KeyForbidden, fallback) }
func TValidationError(fallback string) string   { return T(KeyValidationError, fallback) }
func TNetworkError(fallback string) string      { return T(KeyNetworkError, fallback) }

// Context-aware convenience functions
func TCtxWelcome(c *gin.Context, fallback string) string           { return TCtx(c, KeyWelcome, fallback) }
func TCtxError(c *gin.Context, fallback string) string             { return TCtx(c, KeyError, fallback) }
func TCtxSuccess(c *gin.Context, fallback string) string           { return TCtx(c, KeySuccess, fallback) }
func TCtxWarning(c *gin.Context, fallback string) string           { return TCtx(c, KeyWarning, fallback) }
func TCtxInfo(c *gin.Context, fallback string) string              { return TCtx(c, KeyInfo, fallback) }
func TCtxConfirm(c *gin.Context, fallback string) string           { return TCtx(c, KeyConfirm, fallback) }
func TCtxCancel(c *gin.Context, fallback string) string            { return TCtx(c, KeyCancel, fallback) }
func TCtxSave(c *gin.Context, fallback string) string              { return TCtx(c, KeySave, fallback) }
func TCtxDelete(c *gin.Context, fallback string) string            { return TCtx(c, KeyDelete, fallback) }
func TCtxEdit(c *gin.Context, fallback string) string              { return TCtx(c, KeyEdit, fallback) }
func TCtxAdd(c *gin.Context, fallback string) string               { return TCtx(c, KeyAdd, fallback) }
func TCtxLoading(c *gin.Context, fallback string) string           { return TCtx(c, KeyLoading, fallback) }
func TCtxConnecting(c *gin.Context, fallback string) string        { return TCtx(c, KeyConnecting, fallback) }
func TCtxConnected(c *gin.Context, fallback string) string         { return TCtx(c, KeyConnected, fallback) }
func TCtxDisconnected(c *gin.Context, fallback string) string      { return TCtx(c, KeyDisconnected, fallback) }
func TCtxServerError(c *gin.Context, fallback string) string       { return TCtx(c, KeyServerError, fallback) }
func TCtxNotFound(c *gin.Context, fallback string) string          { return TCtx(c, KeyNotFound, fallback) }
func TCtxUnauthorized(c *gin.Context, fallback string) string      { return TCtx(c, KeyUnauthorized, fallback) }
func TCtxForbidden(c *gin.Context, fallback string) string         { return TCtx(c, KeyForbidden, fallback) }
func TCtxValidationError(c *gin.Context, fallback string) string   { return TCtx(c, KeyValidationError, fallback) }
func TCtxNetworkError(c *gin.Context, fallback string) string      { return TCtx(c, KeyNetworkError, fallback) }