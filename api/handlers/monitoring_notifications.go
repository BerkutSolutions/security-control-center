package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

type notificationChannelPayload struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramChatID    string `json:"telegram_chat_id"`
	TelegramThreadID  *int64 `json:"telegram_thread_id"`
	Silent            bool   `json:"silent"`
	ProtectContent    bool   `json:"protect_content"`
	IsDefault         bool   `json:"is_default"`
	IsActive          *bool  `json:"is_active"`
	ApplyToAll        bool   `json:"apply_to_all"`
}

type notificationChannelView struct {
	ID               int64   `json:"id"`
	Type             string  `json:"type"`
	Name             string  `json:"name"`
	TelegramBotToken string  `json:"telegram_bot_token"`
	TelegramChatID   string  `json:"telegram_chat_id"`
	TelegramThreadID *int64  `json:"telegram_thread_id,omitempty"`
	Silent           bool    `json:"silent"`
	ProtectContent   bool    `json:"protect_content"`
	IsDefault        bool    `json:"is_default"`
	CreatedBy        int64   `json:"created_by"`
	CreatedAt        string  `json:"created_at"`
	IsActive         bool    `json:"is_active"`
}

func (h *MonitoringHandler) ListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListNotificationChannels(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	canManage := hasPermission(r, h.policy, "monitoring.notifications.manage")
	var out []notificationChannelView
	for _, ch := range items {
		tokenMasked := ""
		if len(ch.TelegramBotTokenEnc) > 0 {
			tokenMasked = "******"
		}
		tokenValue := tokenMasked
		if canManage && h.encryptor != nil && len(ch.TelegramBotTokenEnc) > 0 {
			if raw, err := h.encryptor.DecryptBlob(ch.TelegramBotTokenEnc); err == nil {
				tokenValue = string(raw)
			}
		}
		out = append(out, notificationChannelView{
			ID:               ch.ID,
			Type:             ch.Type,
			Name:             ch.Name,
			TelegramBotToken: tokenValue,
			TelegramChatID:   ch.TelegramChatID,
			TelegramThreadID: ch.TelegramThreadID,
			Silent:           ch.Silent,
			ProtectContent:   ch.ProtectContent,
			IsDefault:        ch.IsDefault,
			CreatedBy:        ch.CreatedBy,
			CreatedAt:        ch.CreatedAt.UTC().Format(timeLayout),
			IsActive:         ch.IsActive,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *MonitoringHandler) CreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	if h.encryptor == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload notificationChannelPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.ToLower(strings.TrimSpace(payload.Type)) != "telegram" {
		http.Error(w, "monitoring.notifications.invalidType", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		http.Error(w, "monitoring.notifications.nameRequired", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.TelegramBotToken) == "" || strings.TrimSpace(payload.TelegramChatID) == "" {
		http.Error(w, "monitoring.notifications.telegramRequired", http.StatusBadRequest)
		return
	}
	enc, err := h.encryptor.EncryptToBlob([]byte(strings.TrimSpace(payload.TelegramBotToken)))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}
	ch := &store.NotificationChannel{
		Type:                "telegram",
		Name:                strings.TrimSpace(payload.Name),
		TelegramBotTokenEnc: enc,
		TelegramChatID:      strings.TrimSpace(payload.TelegramChatID),
		TelegramThreadID:    payload.TelegramThreadID,
		Silent:              payload.Silent,
		ProtectContent:      payload.ProtectContent,
		IsDefault:           payload.IsDefault,
		IsActive:            isActive,
		CreatedBy:           sessionUserID(r),
	}
	id, err := h.store.CreateNotificationChannel(r.Context(), ch)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if payload.ApplyToAll {
		h.applyChannelToAllMonitors(r.Context(), id)
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.notification.create", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, notificationChannelView{
		ID:               id,
		Type:             ch.Type,
		Name:             ch.Name,
		TelegramBotToken: maskToken(payload.TelegramBotToken),
		TelegramChatID:   ch.TelegramChatID,
		TelegramThreadID: ch.TelegramThreadID,
		Silent:           ch.Silent,
		ProtectContent:   ch.ProtectContent,
		IsDefault:        ch.IsDefault,
		CreatedBy:        ch.CreatedBy,
		CreatedAt:        ch.CreatedAt.UTC().Format(timeLayout),
		IsActive:         ch.IsActive,
	})
}

func (h *MonitoringHandler) UpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	if h.encryptor == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetNotificationChannel(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload notificationChannelPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if payload.Type != "" && strings.ToLower(strings.TrimSpace(payload.Type)) != "telegram" {
		http.Error(w, "monitoring.notifications.invalidType", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Name) != "" {
		existing.Name = strings.TrimSpace(payload.Name)
	}
	if payload.TelegramChatID != "" {
		existing.TelegramChatID = strings.TrimSpace(payload.TelegramChatID)
	}
	if payload.TelegramThreadID != nil {
		existing.TelegramThreadID = payload.TelegramThreadID
	}
	if payload.TelegramBotToken != "" {
		enc, err := h.encryptor.EncryptToBlob([]byte(strings.TrimSpace(payload.TelegramBotToken)))
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		existing.TelegramBotTokenEnc = enc
	}
	existing.Silent = payload.Silent
	existing.ProtectContent = payload.ProtectContent
	existing.IsDefault = payload.IsDefault
	if payload.IsActive != nil {
		existing.IsActive = *payload.IsActive
	}
	if err := h.store.UpdateNotificationChannel(r.Context(), existing); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.notification.update", strconv.FormatInt(id, 10))
	tokenMasked := "******"
	if payload.TelegramBotToken != "" {
		tokenMasked = maskToken(payload.TelegramBotToken)
	}
	writeJSON(w, http.StatusOK, notificationChannelView{
		ID:               existing.ID,
		Type:             existing.Type,
		Name:             existing.Name,
		TelegramBotToken: tokenMasked,
		TelegramChatID:   existing.TelegramChatID,
		TelegramThreadID: existing.TelegramThreadID,
		Silent:           existing.Silent,
		ProtectContent:   existing.ProtectContent,
		IsDefault:        existing.IsDefault,
		CreatedBy:        existing.CreatedBy,
		CreatedAt:        existing.CreatedAt.UTC().Format(timeLayout),
		IsActive:         existing.IsActive,
	})
}

func (h *MonitoringHandler) DeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteNotificationChannel(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.notification.delete", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) TestNotificationChannel(w http.ResponseWriter, r *http.Request) {
	if h.encryptor == nil || h.engine == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ch, err := h.store.GetNotificationChannel(r.Context(), id)
	if err != nil || ch == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	tokenRaw, err := h.encryptor.DecryptBlob(ch.TelegramBotTokenEnc)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	msg := monitoring.TelegramMessage{
		Token:          string(tokenRaw),
		ChatID:         ch.TelegramChatID,
		ThreadID:       ch.TelegramThreadID,
		Text:           monitoring.NotifyTestMessage("ru"),
		Silent:         ch.Silent,
		ProtectContent: ch.ProtectContent,
	}
	if err := h.engine.TestTelegram(r.Context(), msg); err != nil {
		http.Error(w, "monitoring.notifications.testFailed", http.StatusBadRequest)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.notification.test", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) ListMonitorNotifications(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	items, err := h.store.ListMonitorNotifications(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) UpdateMonitorNotifications(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var payload struct {
		Items []store.MonitorNotification `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	for i := range payload.Items {
		payload.Items[i].MonitorID = id
	}
	if err := h.store.ReplaceMonitorNotifications(r.Context(), id, payload.Items); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.notification.update", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) applyChannelToAllMonitors(ctx context.Context, channelID int64) {
	list, err := h.store.ListMonitors(ctx, store.MonitorFilter{})
	if err != nil {
		return
	}
	for _, mon := range list {
		existing, err := h.store.ListMonitorNotifications(ctx, mon.ID)
		if err != nil {
			continue
		}
		found := false
		for _, item := range existing {
			if item.NotificationChannelID == channelID {
				found = true
				break
			}
		}
		if found {
			continue
		}
		existing = append(existing, store.MonitorNotification{
			MonitorID:            mon.ID,
			NotificationChannelID: channelID,
			Enabled:              true,
		})
		_ = h.store.ReplaceMonitorNotifications(ctx, mon.ID, existing)
	}
}

const timeLayout = "2006-01-02 15:04:05"

func maskToken(token string) string {
	raw := strings.TrimSpace(token)
	if raw == "" {
		return ""
	}
	if len(raw) <= 8 {
		return "******"
	}
	return raw[:4] + "..." + raw[len(raw)-4:]
}
