package backups

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Code    string `json:"code"`
	I18NKey string `json:"i18n_key"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, i18nKey string) {
	writeJSON(w, status, map[string]any{
		"error": apiError{
			Code:    code,
			I18NKey: i18nKey,
		},
	})
}
