// +build !no_embed

package main

import (
	"embed"
)

// WebAssets holds all web assets when embedded
//go:embed web
var WebAssets embed.FS

// UseEmbedded indicates whether to use embedded assets
const UseEmbedded = true