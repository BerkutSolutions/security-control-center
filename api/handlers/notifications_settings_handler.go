package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

const notificationsSettingsModuleID = "notifications.settings"

type NotificationsSettingsHandler struct {
	modules    store.AppModuleStateStore
	audits     store.AuditStore
	monitoring store.MonitoringStore
	engine     *monitoring.Engine
	encryptor  *utils.Encryptor
}

type notificationsSettingsPayload struct {
	MonitoringEnabled bool     `json:"monitoring_enabled"`
	AccessesEnabled   bool     `json:"accesses_enabled"`
	AccessesTypes     []string `json:"accesses_types"`
	AccessesChannelID *int64   `json:"accesses_channel_id,omitempty"`
}

type accessesEventPayload struct {
	EventType     string   `json:"event_type"`
	User          string   `json:"user"`
	Position      string   `json:"position"`
	Department    string   `json:"department"`
	Services      []string `json:"services"`
	Actor         string   `json:"actor"`
	OccurredAt    string   `json:"occurred_at"`
	DismissalDate string   `json:"dismissal_date"`
}

var allowedAccessesNotificationTypes = map[string]struct{}{
	"create":     {},
	"edit":       {},
	"supplement": {},
	"blocked":    {},
	"unblocked":  {},
	"delete":     {},
	"dismissal":  {},
	"test":       {},
}

var defaultAccessesNotificationTypes = []string{"create", "edit", "supplement", "blocked", "unblocked", "delete", "dismissal"}

func NewNotificationsSettingsHandler(
	modules store.AppModuleStateStore,
	audits store.AuditStore,
	monitoringStore store.MonitoringStore,
	engine *monitoring.Engine,
	encryptor *utils.Encryptor,
) *NotificationsSettingsHandler {
	return &NotificationsSettingsHandler{
		modules:    modules,
		audits:     audits,
		monitoring: monitoringStore,
		engine:     engine,
		encryptor:  encryptor,
	}
}

func (h *NotificationsSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h != nil && h.audits != nil {
		_ = h.audits.Log(r.Context(), currentUsername(r), "notifications.settings.view", "")
	}
	writeJSON(w, http.StatusOK, h.loadSettings(r.Context()))
}

func (h *NotificationsSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload notificationsSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	payload.AccessesTypes = sanitizeAccessesNotificationTypes(payload.AccessesTypes)
	if len(payload.AccessesTypes) == 0 {
		payload.AccessesTypes = append([]string{}, defaultAccessesNotificationTypes...)
	}
	if payload.AccessesChannelID != nil && *payload.AccessesChannelID <= 0 {
		payload.AccessesChannelID = nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err = h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               notificationsSettingsModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		_ = h.audits.Log(r.Context(), currentUsername(r), "notifications.settings.update", string(raw))
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *NotificationsSettingsHandler) HandleAccessesEvent(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.monitoring == nil || h.engine == nil || h.encryptor == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	var payload accessesEventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	payload.EventType = strings.ToLower(strings.TrimSpace(payload.EventType))
	if _, ok := allowedAccessesNotificationTypes[payload.EventType]; !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	isTest := payload.EventType == "test"
	settings := h.loadSettings(r.Context())
	if !settings.AccessesEnabled && !isTest {
		h.auditAccessesEvent(r, payload.EventType, "skipped", "disabled")
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "reason": "disabled"})
		return
	}
	if !isTest && !isAccessesTypeEnabled(settings.AccessesTypes, payload.EventType) {
		h.auditAccessesEvent(r, payload.EventType, "skipped", "type_disabled")
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "reason": "type_disabled"})
		return
	}
	if settings.AccessesChannelID == nil || *settings.AccessesChannelID <= 0 {
		h.auditAccessesEvent(r, payload.EventType, "skipped", "channel_missing")
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "reason": "channel_missing"})
		return
	}
	ch, err := h.monitoring.GetNotificationChannel(r.Context(), *settings.AccessesChannelID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if ch == nil || strings.ToLower(strings.TrimSpace(ch.Type)) != "telegram" {
		h.logAccessesDelivery(r.Context(), derefInt64(settings.AccessesChannelID), payload.EventType, "skipped", "channel_invalid", "")
		h.auditAccessesEvent(r, payload.EventType, "skipped", "channel_invalid")
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "reason": "channel_invalid"})
		return
	}
	tokenRaw, err := h.encryptor.DecryptBlob(ch.TelegramBotTokenEnc)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	text := buildAccessesNotificationText(payload)
	msg := monitoring.TelegramMessage{
		Token:          string(tokenRaw),
		ChatID:         ch.TelegramChatID,
		ThreadID:       ch.TelegramThreadID,
		Text:           text,
		Silent:         ch.Silent,
		ProtectContent: ch.ProtectContent,
	}
	sendErr := h.engine.TestTelegram(r.Context(), msg)
	status := "sent"
	errText := ""
	if sendErr != nil {
		status = "failed"
		errText = sendErr.Error()
	}
	h.logAccessesDelivery(r.Context(), ch.ID, payload.EventType, status, errText, text)
	if sendErr != nil {
		h.auditAccessesEvent(r, payload.EventType, "failed", sendErr.Error())
		http.Error(w, "monitoring.notifications.testFailed", http.StatusBadRequest)
		return
	}
	h.auditAccessesEvent(r, payload.EventType, "sent", "")
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (h *NotificationsSettingsHandler) auditAccessesEvent(r *http.Request, eventType, status, reason string) {
	if h == nil || h.audits == nil {
		return
	}
	parts := []string{
		"event=" + strings.TrimSpace(eventType),
		"status=" + strings.TrimSpace(status),
	}
	if strings.TrimSpace(reason) != "" {
		parts = append(parts, "reason="+strings.TrimSpace(reason))
	}
	_ = h.audits.Log(r.Context(), currentUsername(r), "notifications.accesses.send", strings.Join(parts, " "))
}

func (h *NotificationsSettingsHandler) logAccessesDelivery(ctx context.Context, channelID int64, eventType, status, errText, text string) {
	if h == nil || h.monitoring == nil {
		return
	}
	if channelID <= 0 {
		return
	}
	_, _ = h.monitoring.AddNotificationDelivery(ctx, &store.MonitorNotificationDelivery{
		MonitorID:             nil,
		NotificationChannelID: channelID,
		EventType:             "accesses." + strings.TrimSpace(eventType),
		Status:                strings.TrimSpace(status),
		Error:                 strings.TrimSpace(errText),
		BodyPreview:           previewText(text),
	})
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func (h *NotificationsSettingsHandler) loadSettings(ctx context.Context) notificationsSettingsPayload {
	settings := notificationsSettingsPayload{
		MonitoringEnabled: true,
		AccessesEnabled:   true,
		AccessesTypes:     append([]string{}, defaultAccessesNotificationTypes...),
	}
	if h == nil || h.modules == nil {
		return settings
	}
	st, err := h.modules.Get(ctx, notificationsSettingsModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return settings
	}
	if err := json.Unmarshal([]byte(st.LastError), &settings); err != nil {
		return settings
	}
	settings.AccessesTypes = sanitizeAccessesNotificationTypes(settings.AccessesTypes)
	if len(settings.AccessesTypes) == 0 {
		settings.AccessesTypes = append([]string{}, defaultAccessesNotificationTypes...)
	}
	if settings.AccessesChannelID != nil && *settings.AccessesChannelID <= 0 {
		settings.AccessesChannelID = nil
	}
	return settings
}

func sanitizeAccessesNotificationTypes(in []string) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		val := strings.ToLower(strings.TrimSpace(raw))
		if val == "" {
			continue
		}
		if _, ok := allowedAccessesNotificationTypes[val]; !ok {
			continue
		}
		if _, exists := set[val]; exists {
			continue
		}
		set[val] = struct{}{}
		out = append(out, val)
	}
	sort.Strings(out)
	return out
}

func isAccessesTypeEnabled(selected []string, eventType string) bool {
	target := strings.ToLower(strings.TrimSpace(eventType))
	for _, raw := range selected {
		if strings.ToLower(strings.TrimSpace(raw)) == target {
			return true
		}
	}
	return false
}

func buildAccessesNotificationText(payload accessesEventPayload) string {
	lines := []string{
		fmt.Sprintf("Доступы: %s", accessesEventTitle(payload.EventType)),
		fmt.Sprintf("Сотрудник: %s", fallbackText(payload.User)),
	}
	if strings.TrimSpace(payload.Position) != "" {
		lines = append(lines, fmt.Sprintf("Должность: %s", strings.TrimSpace(payload.Position)))
	}
	if strings.TrimSpace(payload.Department) != "" {
		lines = append(lines, fmt.Sprintf("Отдел: %s", strings.TrimSpace(payload.Department)))
	}
	services := normalizeServices(payload.Services)
	if len(services) > 0 {
		lines = append(lines, fmt.Sprintf("Сервисы: %s", strings.Join(services, ", ")))
	}
	if dt, ok := parseDismissalDate(payload.DismissalDate); ok {
		lines = append(lines, fmt.Sprintf("Дата увольнения: %s (%s)", dt.Format("02.01.2006"), russianWeekday(dt.Weekday())))
	}
	occurredAt := parseOccurredAt(payload.OccurredAt)
	lines = append(lines, fmt.Sprintf("Время: %s", formatMoscowTime(occurredAt)))
	if actor := strings.TrimSpace(payload.Actor); actor != "" {
		lines = append(lines, fmt.Sprintf("Инициатор: %s", actor))
	}
	return strings.Join(lines, "\n")
}

func accessesEventTitle(eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "test":
		return "Тест уведомления по доступам"
	case "create":
		return "Создан доступ"
	case "edit":
		return "Отредактирован доступ"
	case "supplement":
		return "Дополнены сервисы"
	case "blocked":
		return "Доступ заблокирован"
	case "unblocked":
		return "Доступ разблокирован"
	case "delete":
		return "Доступ удален"
	case "dismissal":
		return "Увольнение сотрудника"
	default:
		return "Обновление доступа"
	}
}

func fallbackText(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "-"
	}
	return v
}

func normalizeServices(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		val := strings.TrimSpace(raw)
		if val == "" {
			continue
		}
		key := strings.ToLower(val)
		if _, exists := set[key]; exists {
			continue
		}
		set[key] = struct{}{}
		out = append(out, val)
	}
	sort.Strings(out)
	return out
}

func parseDismissalDate(raw string) (time.Time, bool) {
	val := strings.TrimSpace(raw)
	if val == "" {
		return time.Time{}, false
	}
	dt, err := time.Parse("2006-01-02", val)
	if err != nil {
		return time.Time{}, false
	}
	return dt, true
}

func parseOccurredAt(raw string) time.Time {
	val := strings.TrimSpace(raw)
	if val == "" {
		return time.Now().UTC()
	}
	dt, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Now().UTC()
	}
	return dt.UTC()
}

func formatMoscowTime(ts time.Time) string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("MSK", 3*60*60)
	}
	return ts.In(loc).Format("02.01.2006 15:04")
}

func russianWeekday(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "понедельник"
	case time.Tuesday:
		return "вторник"
	case time.Wednesday:
		return "среда"
	case time.Thursday:
		return "четверг"
	case time.Friday:
		return "пятница"
	case time.Saturday:
		return "суббота"
	default:
		return "воскресенье"
	}
}

func previewText(text string) string {
	raw := strings.TrimSpace(text)
	if len(raw) <= 240 {
		return raw
	}
	return raw[:240]
}
