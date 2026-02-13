package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/reports/charts"
	"berkut-scc/core/store"
	"berkut-scc/tasks"
)

func (h *ReportsHandler) CreateFromIncident(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, eff, err := h.currentUserWithAccess(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.policy.Allowed(roles, "incidents.view") {
		http.Error(w, localized(preferredLang(r), "reports.error.forbidden"), http.StatusForbidden)
		return
	}
	var payload struct {
		IncidentID          int64    `json:"incident_id"`
		ClassificationLevel string   `json:"classification_level"`
		ClassificationTags  []string `json:"classification_tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.IncidentID == 0 {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	incident, err := h.incidents.GetIncident(r.Context(), payload.IncidentID)
	if err != nil || incident == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canViewIncidentByClassification(eff, incident.ClassificationLevel, incident.ClassificationTags) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	acl, _ := h.incidents.GetIncidentACL(r.Context(), incident.ID)
	if !h.incidentsSvc.CheckACL(user, roles, acl, "view") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	level := docs.ClassificationLevel(incident.ClassificationLevel)
	tags := docs.NormalizeTags(incident.ClassificationTags)
	if payload.ClassificationLevel != "" {
		if parsed, err := docs.ParseLevel(payload.ClassificationLevel); err == nil {
			if !docs.HasClearance(docs.ClassificationLevel(eff.ClearanceLevel), eff.ClearanceTags, parsed, payload.ClassificationTags) {
				http.Error(w, localized(preferredLang(r), "reports.error.forbidden"), http.StatusForbidden)
				return
			}
			level = parsed
			tags = docs.NormalizeTags(payload.ClassificationTags)
		}
	}
	title := fmt.Sprintf("Incident Report %s", incident.RegNo)
	doc := &store.Document{
		Title:                 title,
		Status:                docs.StatusDraft,
		ClassificationLevel:   int(level),
		ClassificationTags:    tags,
		DocType:               "report",
		InheritACL:            true,
		InheritClassification: true,
		CreatedBy:             user.ID,
		CurrentVersion:        0,
	}
	aclRules := buildBaseACLFor(user)
	docID, err := h.docs.CreateDocument(r.Context(), doc, aclRules, h.cfg.Docs.RegTemplate, h.cfg.Docs.PerFolderSequence)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	doc.ID = docID
	meta := &store.ReportMeta{
		DocID:      docID,
		Status:     "draft",
		PeriodFrom: &incident.CreatedAt,
		PeriodTo:   incident.ClosedAt,
	}
	if meta.PeriodTo == nil {
		now := time.Now().UTC()
		meta.PeriodTo = &now
	}
	if err := h.reports.UpsertReportMeta(r.Context(), meta); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	sections, markdown, snapshotItems, err := h.buildIncidentReport(r.Context(), incident, user, roles, groups, eff)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.reports.ReplaceReportSections(r.Context(), docID, sections); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	_ = h.reports.ReplaceReportCharts(r.Context(), docID, charts.DefaultCharts())
	if _, err := h.svc.SaveVersion(r.Context(), docs.SaveRequest{
		Doc:      doc,
		Author:   user,
		Format:   docs.FormatMarkdown,
		Content:  []byte(markdown),
		Reason:   "incident report",
		IndexFTS: true,
	}); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	snapshotPayload := map[string]any{
		"report_id":    docID,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"incident_id":  incident.ID,
		"incident_reg": incident.RegNo,
	}
	payloadBytes, _ := json.Marshal(snapshotPayload)
	sum := sha256.Sum256(payloadBytes)
	snapshot := &store.ReportSnapshot{
		ReportID:     docID,
		CreatedAt:    time.Now().UTC(),
		CreatedBy:    user.ID,
		Reason:       "incident report",
		Snapshot:     snapshotPayload,
		SnapshotJSON: string(payloadBytes),
		Sha256:       hex.EncodeToString(sum[:]),
	}
	if _, err := h.reports.CreateReportSnapshot(r.Context(), snapshot, snapshotItems); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	link := &store.IncidentLink{
		IncidentID: incident.ID,
		EntityType: "report",
		EntityID:   fmt.Sprintf("%d", docID),
		Title:      doc.Title,
		Unverified: false,
		CreatedBy:  user.ID,
	}
	_, _ = h.incidents.AddIncidentLink(r.Context(), link)
	_, _ = h.incidents.AddIncidentTimeline(r.Context(), &store.IncidentTimelineEvent{
		IncidentID: incident.ID,
		EventType:  "report.create_from_incident",
		Message:    fmt.Sprintf("report %d", docID),
		CreatedBy:  user.ID,
		EventAt:    time.Now().UTC(),
	})
	h.log(r.Context(), user.Username, "incident.report.create", fmt.Sprintf("%s|%d", incident.RegNo, docID))
	h.log(r.Context(), user.Username, "report.create_from_incident", fmt.Sprintf("%s|%d", incident.RegNo, docID))
	writeJSON(w, http.StatusOK, map[string]any{"report_id": docID})
}

func (h *ReportsHandler) buildIncidentReport(ctx context.Context, incident *store.Incident, user *store.User, roles []string, groups []store.Group, eff store.EffectiveAccess) ([]store.ReportSection, string, []store.ReportSnapshotItem, error) {
	var sections []store.ReportSection
	var blocks []string
	var items []store.ReportSnapshotItem
	summary := h.buildIncidentSummarySection(ctx, incident, eff)
	sections = append(sections, store.ReportSection{SectionType: "custom_md", Title: "Summary", IsEnabled: true, Config: map[string]any{"key": "summary", "markdown": summary}})
	blocks = append(blocks, wrapSectionMarkdown("custom_md:summary", summary))
	items = append(items, incidentSnapshotItem(incident))
	timeline := h.buildIncidentTimelineSection(ctx, incident)
	sections = append(sections, store.ReportSection{SectionType: "custom_md", Title: "Timeline", IsEnabled: true, Config: map[string]any{"key": "timeline", "markdown": timeline}})
	blocks = append(blocks, wrapSectionMarkdown("custom_md:timeline", timeline))
	evidence, evidenceItems := h.buildIncidentEvidenceSection(ctx, incident, user, roles, eff)
	sections = append(sections, store.ReportSection{SectionType: "custom_md", Title: "Evidence", IsEnabled: true, Config: map[string]any{"key": "evidence", "markdown": evidence}})
	blocks = append(blocks, wrapSectionMarkdown("custom_md:evidence", evidence))
	items = append(items, evidenceItems...)
	actions, actionItems := h.buildIncidentActionsSection(ctx, incident, user, roles, groups)
	sections = append(sections, store.ReportSection{SectionType: "custom_md", Title: "Actions", IsEnabled: true, Config: map[string]any{"key": "actions", "markdown": actions}})
	blocks = append(blocks, wrapSectionMarkdown("custom_md:actions", actions))
	items = append(items, actionItems...)
	return sections, strings.Join(blocks, "\n\n"), items, nil
}

func (h *ReportsHandler) buildIncidentSummarySection(ctx context.Context, incident *store.Incident, eff store.EffectiveAccess) string {
	var b strings.Builder
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- ID: %s\n", incident.RegNo))
	b.WriteString(fmt.Sprintf("- Title: %s\n", incident.Title))
	b.WriteString(fmt.Sprintf("- Severity: %s\n", incident.Severity))
	b.WriteString(fmt.Sprintf("- Status: %s\n", incident.Status))
	b.WriteString(fmt.Sprintf("- Created: %s\n", incident.CreatedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- Updated: %s\n", incident.UpdatedAt.UTC().Format(time.RFC3339)))
	if incident.ClosedAt != nil {
		b.WriteString(fmt.Sprintf("- Closed: %s\n", incident.ClosedAt.UTC().Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("- Classification: %s\n", docs.LevelName(docs.ClassificationLevel(incident.ClassificationLevel))))
	if len(incident.ClassificationTags) > 0 {
		b.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(docs.NormalizeTags(incident.ClassificationTags), ", ")))
	}
	if strings.TrimSpace(incident.Description) != "" {
		b.WriteString("\n")
		b.WriteString(incident.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func (h *ReportsHandler) buildIncidentTimelineSection(ctx context.Context, incident *store.Incident) string {
	var b strings.Builder
	b.WriteString("## Timeline\n\n")
	timeline, _ := h.incidents.ListIncidentTimeline(ctx, incident.ID, 50, "")
	if len(timeline) == 0 {
		b.WriteString("_No timeline events._\n")
		return b.String()
	}
	b.WriteString("| Time | Type | Message |\n|---|---|---|\n")
	for _, ev := range timeline {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			ev.EventAt.UTC().Format("2006-01-02 15:04"),
			escapePipes(ev.EventType),
			escapePipes(ev.Message),
		))
	}
	return b.String()
}

func (h *ReportsHandler) buildIncidentEvidenceSection(ctx context.Context, incident *store.Incident, user *store.User, roles []string, eff store.EffectiveAccess) (string, []store.ReportSnapshotItem) {
	var b strings.Builder
	var items []store.ReportSnapshotItem
	b.WriteString("## Evidence\n\n")
	atts, _ := h.incidents.ListIncidentAttachments(ctx, incident.ID)
	var visible []store.IncidentAttachment
	for _, att := range atts {
		if h.canViewIncidentByClassification(eff, att.ClassificationLevel, att.ClassificationTags) {
			visible = append(visible, att)
		}
	}
	if len(visible) == 0 {
		b.WriteString("_No attachments._\n")
	} else {
		b.WriteString("| File | Size | Uploaded |\n|---|---|---|\n")
		for _, att := range visible {
			b.WriteString(fmt.Sprintf("| %s | %d | %s |\n",
				escapePipes(att.Filename),
				att.SizeBytes,
				att.UploadedAt.UTC().Format("2006-01-02 15:04"),
			))
			items = append(items, store.ReportSnapshotItem{
				EntityType: "incident_attachment",
				EntityID:   fmt.Sprintf("%d", att.ID),
				Entity: map[string]any{
					"id":          att.ID,
					"filename":    att.Filename,
					"size_bytes":  att.SizeBytes,
					"uploaded_at": att.UploadedAt.UTC().Format(time.RFC3339),
				},
			})
		}
	}
	links, _ := h.incidents.ListIncidentLinks(ctx, incident.ID)
	if len(links) > 0 {
		b.WriteString("\n### Linked documents\n\n")
		b.WriteString("| Type | Title | ID |\n|---|---|---|\n")
		for _, link := range links {
			if link.EntityType != "doc" && link.EntityType != "report" {
				continue
			}
			id := parseInt64Default(link.EntityID, 0)
			if id == 0 {
				continue
			}
			doc, _ := h.docs.GetDocument(ctx, id)
			if doc == nil {
				continue
			}
			docACL, _ := h.docs.GetDocACL(ctx, doc.ID)
			var folderACL []store.ACLRule
			if doc.FolderID != nil {
				folderACL, _ = h.docs.GetFolderACL(ctx, *doc.FolderID)
			}
			if !h.svc.CheckACL(user, roles, doc, docACL, folderACL, "view") {
				continue
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %d |\n",
				escapePipes(link.EntityType),
				escapePipes(doc.Title),
				doc.ID,
			))
			items = append(items, store.ReportSnapshotItem{
				EntityType: "doc",
				EntityID:   fmt.Sprintf("%d", doc.ID),
				Entity: map[string]any{
					"id":    doc.ID,
					"title": doc.Title,
					"type":  link.EntityType,
				},
			})
		}
	}
	if incident.Source != "" && incident.SourceRefID != nil && *incident.SourceRefID > 0 && h.monitoring != nil {
		events, _ := h.monitoring.ListEvents(ctx, *incident.SourceRefID, incident.CreatedAt.UTC())
		if len(events) > 0 {
			b.WriteString("\n### Monitoring events\n\n| Time | Type | Message |\n|---|---|---|\n")
			for _, ev := range events {
				b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					ev.TS.UTC().Format("2006-01-02 15:04"),
					escapePipes(ev.EventType),
					escapePipes(ev.Message),
				))
				items = append(items, store.ReportSnapshotItem{
					EntityType: "monitor_event",
					EntityID:   fmt.Sprintf("%d", ev.ID),
					Entity: map[string]any{
						"id":         ev.ID,
						"monitor_id": ev.MonitorID,
						"event_type": ev.EventType,
						"message":    ev.Message,
						"ts":         ev.TS.UTC().Format(time.RFC3339),
					},
				})
			}
		}
	}
	return b.String(), items
}

func (h *ReportsHandler) buildIncidentActionsSection(ctx context.Context, incident *store.Incident, user *store.User, roles []string, groups []store.Group) (string, []store.ReportSnapshotItem) {
	var b strings.Builder
	var items []store.ReportSnapshotItem
	b.WriteString("## Actions\n\n")
	links, _ := h.incidents.ListIncidentLinks(ctx, incident.ID)
	var taskIDs []int64
	for _, link := range links {
		if link.EntityType != "task" {
			continue
		}
		if id := parseInt64Default(link.EntityID, 0); id > 0 {
			taskIDs = append(taskIDs, id)
		}
	}
	if len(taskIDs) == 0 {
		b.WriteString("_No linked tasks._\n")
		return b.String(), items
	}
	assignments, _ := h.tasksSvc.Store().ListTaskAssignmentsForTasks(ctx, taskIDs)
	userCache := map[int64]string{}
	b.WriteString("| Task | Status | Assignees |\n|---|---|---|\n")
	for _, id := range taskIDs {
		task, _ := h.tasksSvc.Store().GetTask(ctx, id)
		if task == nil {
			continue
		}
		board, _ := h.tasksSvc.Store().GetBoard(ctx, task.BoardID)
		var spaceACL []tasks.ACLRule
		if board != nil && board.SpaceID > 0 {
			spaceACL, _ = h.tasksSvc.Store().GetSpaceACL(ctx, board.SpaceID)
		}
		boardACL, _ := h.tasksSvc.Store().GetBoardACL(ctx, task.BoardID)
		if !taskBoardAllowed(user, roles, groups, spaceACL, boardACL, "view") {
			continue
		}
		assignees := taskAssignees(assignments[id], userCache, h)
		b.WriteString(fmt.Sprintf("| %d | %s | %s |\n", task.ID, escapePipes(task.Status), escapePipes(assignees)))
		items = append(items, store.ReportSnapshotItem{
			EntityType: "task",
			EntityID:   fmt.Sprintf("%d", task.ID),
			Entity: map[string]any{
				"id":       task.ID,
				"title":    task.Title,
				"status":   task.Status,
				"board_id": task.BoardID,
			},
		})
	}
	return b.String(), items
}

func incidentSnapshotItem(incident *store.Incident) store.ReportSnapshotItem {
	return store.ReportSnapshotItem{
		EntityType: "incident",
		EntityID:   fmt.Sprintf("%d", incident.ID),
		Entity: map[string]any{
			"id":         incident.ID,
			"reg_no":     incident.RegNo,
			"title":      incident.Title,
			"severity":   incident.Severity,
			"status":     incident.Status,
			"created_at": incident.CreatedAt.UTC().Format(time.RFC3339),
			"updated_at": incident.UpdatedAt.UTC().Format(time.RFC3339),
		},
	}
}
