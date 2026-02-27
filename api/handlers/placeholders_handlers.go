package handlers

import (
	"bytes"
	"net/http"
	"path/filepath"
	"time"

	"berkut-scc/core/rbac"
	"berkut-scc/gui"
)

var pageFiles = map[string]string{
	"docs":       "docs.html",
	"approvals":  "approvals.html",
	"logs":       "logs.html",
	"controls":   "controls.html",
	"registry":   "controls.html",
	"assets":     "assets.html",
	"software":   "software.html",
	"backups":    "backups.html",
	"tasks":      "tasks.html",
	"findings":   "findings.html",
	"incidents":  "incidents.html",
	"reports":    "reports.html",
	"monitoring": "monitoring.html",
}

type PlaceholderHandler struct{}

func NewPlaceholderHandler() *PlaceholderHandler {
	return &PlaceholderHandler{}
}

func (h *PlaceholderHandler) Page(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	file, ok := pageFiles[name]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data, err := gui.StaticFiles.ReadFile(filepath.Join("static", file))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, file, time.Now(), bytes.NewReader(data))
}

func RequiredPermission(name string) rbac.Permission {
	switch name {
	case "docs":
		return "docs.view"
	case "approvals":
		return "docs.approval.view"
	case "controls":
		return "controls.view"
	case "registry":
		return "controls.view"
	case "assets":
		return "assets.view"
	case "software":
		return "software.view"
	case "backups":
		return "backups.read"
	case "tasks":
		return "tasks.view"
	case "findings":
		return "findings.view"
	case "incidents":
		return "incidents.view"
	case "reports":
		return "reports.view"
	case "monitoring":
		return "monitoring.view"
	case "logs":
		return "logs.view"
	default:
		return ""
	}
}
