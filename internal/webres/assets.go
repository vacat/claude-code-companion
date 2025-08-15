package webres

import (
	"html/template"
	"io/fs"
)

// AssetProvider defines the interface for accessing web assets
type AssetProvider interface {
	GetTemplateFS() (fs.FS, error)
	GetStaticFS() (fs.FS, error) 
	GetLocalesFS() (fs.FS, error)
	LoadTemplates() (*template.Template, error)
	ReadLocaleFile(filename string) ([]byte, error)
}

// Default provider instance
var provider AssetProvider

// SetProvider sets the global asset provider
func SetProvider(p AssetProvider) {
	provider = p
}

// GetTemplateFS returns the template filesystem
func GetTemplateFS() (fs.FS, error) {
	if provider != nil {
		return provider.GetTemplateFS()
	}
	return nil, fs.ErrNotExist
}

// GetStaticFS returns the static filesystem  
func GetStaticFS() (fs.FS, error) {
	if provider != nil {
		return provider.GetStaticFS()
	}
	return nil, fs.ErrNotExist
}

// GetLocalesFS returns the locales filesystem
func GetLocalesFS() (fs.FS, error) {
	if provider != nil {
		return provider.GetLocalesFS()
	}
	return nil, fs.ErrNotExist
}

// LoadTemplates loads all templates
func LoadTemplates() (*template.Template, error) {
	if provider != nil {
		return provider.LoadTemplates()
	}
	return nil, fs.ErrNotExist
}

// ReadLocaleFile reads a locale file
func ReadLocaleFile(filename string) ([]byte, error) {
	if provider != nil {
		return provider.ReadLocaleFile(filename)
	}
	return nil, fs.ErrNotExist
}