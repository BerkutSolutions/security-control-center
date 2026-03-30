package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) LogSecurityEvent(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	var payload struct {
		EventType string `json:"event_type"`
		Details   string `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	eventType := strings.ToLower(strings.TrimSpace(payload.EventType))
	switch eventType {
	case "copy_blocked", "screenshot_attempt":
	default:
		http.Error(w, "reports.security.invalidEvent", http.StatusBadRequest)
		return
	}
	h.log(r.Context(), user.Username, "report.security."+eventType, fmt.Sprintf("%s|%s", doc.RegNumber, strings.TrimSpace(payload.Details)))
	if h.behavior != nil {
		evt := "dlp.copy_blocked"
		if eventType == "screenshot_attempt" {
			evt = "dlp.screenshot_attempt"
		}
		_ = h.behavior.RecordEvent(r.Context(), &store.BehaviorRiskEvent{
			UserID:     user.ID,
			EventType:  evt,
			Path:       "/api/reports/" + strconv.FormatInt(doc.ID, 10) + "/security-events",
			Method:     http.MethodPost,
			StatusCode: http.StatusOK,
			IP:         reportsSecurityRemoteIP(r),
			CreatedAt:  time.Now().UTC(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func reportsSecurityRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	raw := strings.TrimSpace(r.RemoteAddr)
	if raw == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.TrimSpace(host)
	}
	return raw
}
