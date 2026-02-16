package handlers

import (
	"bytes"
	"net/http"
	"time"

	"berkut-scc/gui"
)

type SettingsHandler struct{}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

func (h *SettingsHandler) Page(w http.ResponseWriter, r *http.Request) {
	data, err := gui.StaticFiles.ReadFile("static/settings.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, "settings.html", time.Now(), bytes.NewReader(data))
}
