package handlers

import (
	"encoding/json"
	"net/http"
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
