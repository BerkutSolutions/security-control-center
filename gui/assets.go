package gui

import "embed"

//go:embed static/* static/placeholders/* static/js/* static/i18n/* static/styles.css
var StaticFiles embed.FS
