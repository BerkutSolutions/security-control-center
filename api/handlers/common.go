package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	SessionCookieName = "berkut_session"
	CSRFCookieName    = "berkut_csrf"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseMultipartFormLimited(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return err
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return err
	}
	return nil
}

func attachmentDisposition(filename string) string {
	return fmt.Sprintf("attachment; filename=%q", sanitizeHeaderFilename(filename))
}

func sanitizeHeaderFilename(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "file"
	}
	replacer := strings.NewReplacer(
		"\r", "",
		"\n", "",
		"\"", "",
		";", "_",
		"/", "_",
		"\\", "_",
	)
	clean = replacer.Replace(clean)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "file"
	}
	return clean
}
