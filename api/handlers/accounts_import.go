package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type importOptions struct {
	TempPassword       *bool `json:"temp_password"`
	MustChangePassword *bool `json:"must_change_password"`
}

type importDefaults struct {
	DefaultRole  string `json:"default_role"`
	DefaultGroup string `json:"default_group"`
}

type importCommitPayload struct {
	ImportID string            `json:"import_id"`
	Mapping  map[string]string `json:"mapping"`
	Options  importOptions     `json:"options"`
	Defaults importDefaults    `json:"defaults"`
}

type importFailure struct {
	RowNumber int    `json:"row_number"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail,omitempty"`
}

type importCreatedUser struct {
	Login        string `json:"login"`
	TempPassword string `json:"temp_password,omitempty"`
}

type importReport struct {
	TotalRows    int                 `json:"total_rows"`
	CreatedCount int                 `json:"created_count"`
	UpdatedCount int                 `json:"updated_count"`
	FailedCount  int                 `json:"failed_count"`
	Failures     []importFailure     `json:"failures"`
	CreatedUsers []importCreatedUser `json:"created_users,omitempty"`
}

type userImportSession struct {
	ID         string
	Headers    []string
	Normalized []string
	Rows       [][]string
	CreatedAt  time.Time
	Filename   string
}

func (s *userImportSession) headerIndex() map[string]int {
	index := make(map[string]int, len(s.Normalized))
	for i, h := range s.Normalized {
		index[h] = i
	}
	return index
}

func (s *userImportSession) preview(limit int) [][]string {
	if limit <= 0 || len(s.Rows) <= limit {
		return s.Rows
	}
	return s.Rows[:limit]
}

type userImportManager struct {
	mu       sync.Mutex
	sessions map[string]*userImportSession
	ttl      time.Duration
}

func newUserImportManager() *userImportManager {
	return &userImportManager{
		sessions: make(map[string]*userImportSession),
		ttl:      15 * time.Minute,
	}
}

func (m *userImportManager) save(session *userImportSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	m.sessions[session.ID] = session
}

func (m *userImportManager) get(id string) (*userImportSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	sess, ok := m.sessions[id]
	return sess, ok
}

func (m *userImportManager) delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

func (m *userImportManager) cleanupLocked() {
	now := time.Now().UTC()
	for id, sess := range m.sessions {
		if now.Sub(sess.CreatedAt) > m.ttl {
			delete(m.sessions, id)
		}
	}
}

func (h *AccountsHandler) ImportUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	if err := parseMultipartFormLimited(w, r, 20<<20); err != nil {
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 {
		http.Error(w, "invalid csv", http.StatusBadRequest)
		return
	}
	headers := trimHeaders(rows[0])
	if len(headers) == 0 {
		http.Error(w, "no headers", http.StatusBadRequest)
		return
	}
	cleanRows := make([][]string, 0, len(rows)-1)
	for _, raw := range rows[1:] {
		cleanRows = append(cleanRows, trimRow(raw, len(headers)))
	}
	id, _ := utils.RandString(12)
	session := &userImportSession{
		ID:         id,
		Headers:    headers,
		Normalized: normalizeHeaders(headers),
		Rows:       cleanRows,
		CreatedAt:  time.Now().UTC(),
		Filename:   hdr.Filename,
	}
	h.imports.save(session)
	h.audits.Log(ctx, currentUser(r), "accounts.import_start", fmt.Sprintf("%s|%d", id, len(cleanRows)))
	writeJSON(w, http.StatusOK, map[string]any{
		"import_id":        id,
		"detected_headers": headers,
		"preview_rows":     session.preview(10),
	})
}

func (h *AccountsHandler) ImportCommit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	var payload importCommitPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if payload.ImportID == "" {
		http.Error(w, "import_id required", http.StatusBadRequest)
		return
	}
	session, ok := h.imports.get(payload.ImportID)
	if !ok || session == nil {
		http.Error(w, "import not found", http.StatusNotFound)
		return
	}
	defer h.imports.delete(payload.ImportID)

	if payload.Mapping == nil {
		http.Error(w, "mapping required", http.StatusBadRequest)
		return
	}
	required := []string{"login", "full_name"}
	for _, field := range required {
		if strings.TrimSpace(payload.Mapping[field]) == "" {
			http.Error(w, "mapping missing required fields", http.StatusBadRequest)
			return
		}
	}

	tempPassword := true
	if payload.Options.TempPassword != nil {
		tempPassword = *payload.Options.TempPassword
	}
	mustChange := true
	if payload.Options.MustChangePassword != nil {
		mustChange = *payload.Options.MustChangePassword
	}

	actorName := currentUser(r)
	actor, actorRoles, _ := h.users.FindByUsername(ctx, actorName)
	if actor == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	actorEff, _ := h.effectiveAccess(ctx, actor, actorRoles)
	actorForTags := &store.User{ClearanceTags: actorEff.ClearanceTags}
	sessionRec := sessionFromCtx(r)

	rolesList, _ := h.roles.List(ctx)
	roleSet := map[string]struct{}{}
	for _, rl := range rolesList {
		roleSet[strings.ToLower(strings.TrimSpace(rl.Name))] = struct{}{}
	}
	groupsList, _ := h.groups.List(ctx)
	groupSet := map[string]store.Group{}
	for _, g := range groupsList {
		groupSet[strings.ToLower(strings.TrimSpace(g.Name))] = g
	}
	if _, err := resolveDefaultGroup(payload.Defaults.DefaultGroup, groupSet); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defaultRole := strings.ToLower(strings.TrimSpace(payload.Defaults.DefaultRole))
	if defaultRole != "" {
		if _, ok := roleSet[defaultRole]; !ok {
			http.Error(w, "default role not found", http.StatusBadRequest)
			return
		}
	}

	headerIndex := session.headerIndex()
	existingUsers, _ := h.users.List(ctx)
	existing := map[string]store.UserWithRoles{}
	for _, u := range existingUsers {
		existing[strings.ToLower(u.Username)] = u
	}

	report := importReport{TotalRows: len(session.Rows)}
	seen := map[string]struct{}{}
	for i, row := range session.Rows {
		rowNum := i + 2 // account for header row
		val := func(field string) string {
			header := strings.ToLower(strings.TrimSpace(payload.Mapping[field]))
			if header == "" {
				return ""
			}
			idx, ok := headerIndex[header]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		login := strings.ToLower(val("login"))
		if login == "" {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "login"})
			continue
		}
		if err := utils.ValidateUsername(login); err != nil {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "login"})
			continue
		}
		if _, dup := seen[login]; dup {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "already_exists"})
			continue
		}
		seen[login] = struct{}{}
		if _, exists := existing[login]; exists {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "already_exists"})
			continue
		}
		if strings.EqualFold(login, actorName) {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "self"})
			continue
		}

		fullName := strings.TrimSpace(val("full_name"))
		if fullName == "" {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "full_name"})
			continue
		}
		email := strings.TrimSpace(val("email"))
		department := strings.TrimSpace(val("department"))
		position := strings.TrimSpace(val("position"))
		statusVal := strings.ToLower(val("status"))
		active := true
		var disabledAt *time.Time
		if statusVal != "" && statusVal != "active" {
			if statusVal == "disabled" || statusVal == "inactive" {
				active = false
				now := time.Now().UTC()
				disabledAt = &now
			} else {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "status"})
				continue
			}
		}

		clearanceLevel := 0
		if clVal := strings.TrimSpace(val("clearance_level")); clVal != "" {
			n, err := strconv.Atoi(clVal)
			if err != nil || n < 0 {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "clearance_level"})
				continue
			}
			clearanceLevel = n
		}
		tags := sanitizeTags(splitAndTrim(val("clearance_tags")))
		if clearanceLevel > actorEff.ClearanceLevel {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "clearance_too_high"})
			continue
		}
		if !h.canAssignTags(actorForTags, tags) {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "clearance_too_high"})
			continue
		}

		roleValues := splitAndTrim(val("roles"))
		roles := sanitizeRoles(roleValues, defaultRole)
		if len(roles) == 0 {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "roles"})
			continue
		}
		if containsRole(roles, "superadmin") && (sessionRec == nil || !containsRole(sessionRec.Roles, "superadmin")) {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "roles"})
			continue
		}
		rolesValid := true
		for _, rname := range roles {
			if _, ok := roleSet[strings.ToLower(rname)]; !ok {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "role_not_found", Detail: rname})
				rolesValid = false
				break
			}
		}
		if !rolesValid {
			continue
		}

		groupNames := splitAndTrim(val("groups"))
		if payload.Defaults.DefaultGroup != "" {
			groupNames = append(groupNames, payload.Defaults.DefaultGroup)
		}
		groupIDs := uniqueGroupIDs(groupNames, groupSet)
		if groupIDs == nil && len(groupNames) > 0 {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "group_not_found"})
			continue
		}
		if err := h.validateGroupAssignments(ctx, actorEff, groupIDs); err != nil {
			if err.Error() == "clearance_too_high" || err.Error() == "clearance_tags_not_allowed" {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "clearance_too_high"})
			} else {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "groups"})
			}
			continue
		}

		passwordSet := false
		requireChange := mustChange
		hash := ""
		salt := ""
		var err error
		tempPwd := ""
		if tempPassword {
			tempPwd, err = utils.RandString(18)
			if err != nil {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "password"})
				continue
			}
			ph, err := auth.HashPassword(tempPwd, h.cfg.Pepper)
			if err != nil {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "password"})
				continue
			}
			hash = ph.Hash
			salt = ph.Salt
			passwordSet = true
		}

		user := &store.User{
			Username:              login,
			Email:                 email,
			FullName:              fullName,
			Department:            department,
			Position:              position,
			ClearanceLevel:        clearanceLevel,
			ClearanceTags:         tags,
			PasswordHash:          hash,
			Salt:                  salt,
			PasswordSet:           passwordSet,
			RequirePasswordChange: requireChange || !passwordSet,
			Active:                active,
			DisabledAt:            disabledAt,
		}
		userID, err := h.users.Create(ctx, user, roles)
		if err != nil {
			report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "create"})
			continue
		}
		if len(groupIDs) > 0 {
			if err := h.groups.SetUserGroups(ctx, userID, groupIDs); err != nil {
				report.Failures = append(report.Failures, importFailure{RowNumber: rowNum, Reason: "invalid_value", Detail: "groups"})
				continue
			}
		}
		report.CreatedCount++
		created := importCreatedUser{Login: login}
		if tempPassword {
			created.TempPassword = tempPwd
		}
		report.CreatedUsers = append(report.CreatedUsers, created)
		h.audits.Log(ctx, actorName, "accounts.user_created_via_import", login)
	}

	report.UpdatedCount = 0
	report.FailedCount = len(report.Failures)
	if report.FailedCount > 0 {
		h.audits.Log(ctx, actorName, "accounts.import_row_failed", fmt.Sprintf("%s|%d", payload.ImportID, report.FailedCount))
	}
	h.audits.Log(ctx, actorName, "accounts.import_commit", fmt.Sprintf("%d|%d|%d|%d", report.TotalRows, report.CreatedCount, report.UpdatedCount, report.FailedCount))
	writeJSON(w, http.StatusOK, report)
}

func trimHeaders(headers []string) []string {
	out := make([]string, len(headers))
	for i, h := range headers {
		val := strings.TrimSpace(h)
		if val == "" {
			val = fmt.Sprintf("col_%d", i+1)
		}
		out[i] = val
	}
	return out
}

func trimRow(row []string, expected int) []string {
	out := make([]string, expected)
	for i := 0; i < expected; i++ {
		if i < len(row) {
			out[i] = strings.TrimSpace(row[i])
		} else {
			out[i] = ""
		}
	}
	return out
}

func splitAndTrim(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ';' || r == ',' || r == '|'
	})
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		res = append(res, p)
	}
	return res
}

func uniqueGroupIDs(names []string, groupSet map[string]store.Group) []int64 {
	if len(names) == 0 {
		return []int64{}
	}
	ids := []int64{}
	seen := map[int64]struct{}{}
	for _, name := range names {
		g, ok := groupSet[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil
		}
		if _, exists := seen[g.ID]; exists {
			continue
		}
		seen[g.ID] = struct{}{}
		ids = append(ids, g.ID)
	}
	return ids
}

func resolveDefaultGroup(name string, groupSet map[string]store.Group) ([]int64, error) {
	if strings.TrimSpace(name) == "" {
		return []int64{}, nil
	}
	g, ok := groupSet[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, fmt.Errorf("default group not found")
	}
	return []int64{g.ID}, nil
}
