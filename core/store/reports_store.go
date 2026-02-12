package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type ReportMeta struct {
	DocID      int64      `json:"doc_id"`
	PeriodFrom *time.Time `json:"period_from,omitempty"`
	PeriodTo   *time.Time `json:"period_to,omitempty"`
	Status     string     `json:"report_status"`
	TemplateID *int64     `json:"template_id,omitempty"`
}

type ReportTemplate struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	TemplateMarkdown string   `json:"template_markdown"`
	CreatedBy       int64     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReportSettings struct {
	ID                   int64      `json:"id"`
	DefaultClassification string     `json:"default_classification"`
	DefaultTemplateID     *int64     `json:"default_template_id,omitempty"`
	HeaderEnabled         bool       `json:"header_enabled"`
	HeaderLogoPath        string     `json:"header_logo_path"`
	HeaderTitle           string     `json:"header_title"`
	WatermarkThreshold    string     `json:"watermark_threshold"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ReportFilter struct {
	Search             string
	MineUserID         int64
	Status             string
	ClassificationLevel int
	Tags               []string
	DateFrom           *time.Time
	DateTo             *time.Time
	Limit              int
	Offset             int
}

type ReportRecord struct {
	Document Document   `json:"document"`
	Meta     ReportMeta `json:"meta"`
}

type ReportsStore interface {
	GetReportMeta(ctx context.Context, docID int64) (*ReportMeta, error)
	UpsertReportMeta(ctx context.Context, meta *ReportMeta) error
	ListReports(ctx context.Context, filter ReportFilter) ([]ReportRecord, error)

	ListReportTemplates(ctx context.Context) ([]ReportTemplate, error)
	GetReportTemplate(ctx context.Context, id int64) (*ReportTemplate, error)
	SaveReportTemplate(ctx context.Context, tpl *ReportTemplate) error
	DeleteReportTemplate(ctx context.Context, id int64) error
	EnsureDefaultReportTemplates(ctx context.Context) error

	GetReportSettings(ctx context.Context) (*ReportSettings, error)
	UpdateReportSettings(ctx context.Context, settings *ReportSettings) error

	ListReportSections(ctx context.Context, reportID int64) ([]ReportSection, error)
	ReplaceReportSections(ctx context.Context, reportID int64, sections []ReportSection) error
	ListReportCharts(ctx context.Context, reportID int64) ([]ReportChart, error)
	GetReportChart(ctx context.Context, chartID int64) (*ReportChart, error)
	ReplaceReportCharts(ctx context.Context, reportID int64, charts []ReportChart) error
	CreateReportSnapshot(ctx context.Context, snapshot *ReportSnapshot, items []ReportSnapshotItem) (int64, error)
	ListReportSnapshots(ctx context.Context, reportID int64) ([]ReportSnapshot, error)
	GetReportSnapshot(ctx context.Context, snapshotID int64) (*ReportSnapshot, []ReportSnapshotItem, error)
}

type reportsStore struct {
	db *sql.DB
}

func NewReportsStore(db *sql.DB) ReportsStore {
	return &reportsStore{db: db}
}

func (s *reportsStore) GetReportMeta(ctx context.Context, docID int64) (*ReportMeta, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT doc_id, period_from, period_to, report_status, template_id
		FROM report_meta WHERE doc_id=?`, docID)
	return scanReportMeta(row)
}

func (s *reportsStore) UpsertReportMeta(ctx context.Context, meta *ReportMeta) error {
	if meta == nil {
		return errors.New("missing meta")
	}
	if strings.TrimSpace(meta.Status) == "" {
		meta.Status = "draft"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO report_meta(doc_id, period_from, period_to, report_status, template_id)
		VALUES(?,?,?,?,?)
		ON CONFLICT(doc_id) DO UPDATE SET period_from=excluded.period_from, period_to=excluded.period_to, report_status=excluded.report_status, template_id=excluded.template_id`,
		meta.DocID, meta.PeriodFrom, meta.PeriodTo, meta.Status, nullableID(meta.TemplateID))
	return err
}

func (s *reportsStore) ListReports(ctx context.Context, filter ReportFilter) ([]ReportRecord, error) {
	clauses := []string{"d.doc_type='report'", "d.deleted_at IS NULL"}
	var args []any
	if filter.Search != "" {
		clauses = append(clauses, "(d.title LIKE ? OR d.reg_number LIKE ?)")
		q := "%" + filter.Search + "%"
		args = append(args, q, q)
	}
	if filter.MineUserID > 0 {
		clauses = append(clauses, "d.created_by=?")
		args = append(args, filter.MineUserID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "rm.report_status=?")
		args = append(args, filter.Status)
	}
	if filter.ClassificationLevel >= 0 {
		clauses = append(clauses, "d.classification_level=?")
		args = append(args, filter.ClassificationLevel)
	}
	for _, t := range filter.Tags {
		if strings.TrimSpace(t) == "" {
			continue
		}
		clauses = append(clauses, "d.classification_tags LIKE ?")
		args = append(args, "%"+strings.ToUpper(strings.TrimSpace(t))+"%")
	}
	if filter.DateFrom != nil {
		clauses = append(clauses, "rm.period_from IS NOT NULL AND rm.period_from>=?")
		args = append(args, filter.DateFrom)
	}
	if filter.DateTo != nil {
		clauses = append(clauses, "rm.period_to IS NOT NULL AND rm.period_to<=?")
		args = append(args, filter.DateTo)
	}
	where := " WHERE " + strings.Join(clauses, " AND ")
	limitOffset := ""
	if filter.Limit > 0 {
		limitOffset = " LIMIT " + strconv.Itoa(filter.Limit)
		if filter.Offset > 0 {
			limitOffset += " OFFSET " + strconv.Itoa(filter.Offset)
		}
	}
	query := `
		SELECT d.id, d.folder_id, d.title, d.status, d.classification_level, d.classification_tags, d.reg_number, d.doc_type, d.inherit_acl, d.inherit_classification, d.created_by, d.current_version, d.created_at, d.updated_at, d.deleted_at,
		       rm.period_from, rm.period_to, rm.report_status, rm.template_id
		FROM docs d
		LEFT JOIN report_meta rm ON rm.doc_id=d.id` + where + ` ORDER BY d.updated_at DESC` + limitOffset
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportRecord
	for rows.Next() {
		var doc Document
		var folder sql.NullInt64
		var tagsRaw string
		var deleted sql.NullTime
		var meta ReportMeta
		var periodFrom, periodTo sql.NullTime
		var template sql.NullInt64
		if err := rows.Scan(&doc.ID, &folder, &doc.Title, &doc.Status, &doc.ClassificationLevel, &tagsRaw, &doc.RegNumber, &doc.DocType, &doc.InheritACL, &doc.InheritClassification, &doc.CreatedBy, &doc.CurrentVersion, &doc.CreatedAt, &doc.UpdatedAt, &deleted,
			&periodFrom, &periodTo, &meta.Status, &template); err != nil {
			return nil, err
		}
		if folder.Valid {
			doc.FolderID = &folder.Int64
		}
		if deleted.Valid {
			doc.DeletedAt = &deleted.Time
		}
		if tagsRaw != "" {
			_ = json.Unmarshal([]byte(tagsRaw), &doc.ClassificationTags)
		}
		meta.DocID = doc.ID
		if periodFrom.Valid {
			meta.PeriodFrom = &periodFrom.Time
		}
		if periodTo.Valid {
			meta.PeriodTo = &periodTo.Time
		}
		if template.Valid {
			meta.TemplateID = &template.Int64
		}
		if strings.TrimSpace(meta.Status) == "" {
			meta.Status = "draft"
		}
		res = append(res, ReportRecord{Document: doc, Meta: meta})
	}
	return res, rows.Err()
}

func (s *reportsStore) ListReportTemplates(ctx context.Context) ([]ReportTemplate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, template_markdown, created_by, created_at, updated_at
		FROM report_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportTemplate
	for rows.Next() {
		var tpl ReportTemplate
		if err := rows.Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.TemplateMarkdown, &tpl.CreatedBy, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, tpl)
	}
	return res, rows.Err()
}

func (s *reportsStore) GetReportTemplate(ctx context.Context, id int64) (*ReportTemplate, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, template_markdown, created_by, created_at, updated_at
		FROM report_templates WHERE id=?`, id)
	var tpl ReportTemplate
	if err := row.Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.TemplateMarkdown, &tpl.CreatedBy, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tpl, nil
}

func (s *reportsStore) SaveReportTemplate(ctx context.Context, tpl *ReportTemplate) error {
	if tpl == nil {
		return errors.New("missing template")
	}
	now := time.Now().UTC()
	if tpl.ID == 0 {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO report_templates(name, description, template_markdown, created_by, created_at, updated_at)
			VALUES(?,?,?,?,?,?)`,
			tpl.Name, tpl.Description, tpl.TemplateMarkdown, tpl.CreatedBy, now, now)
		if err != nil {
			return err
		}
		tpl.ID, _ = res.LastInsertId()
		tpl.CreatedAt = now
		tpl.UpdatedAt = now
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE report_templates SET name=?, description=?, template_markdown=?, updated_at=? WHERE id=?`,
		tpl.Name, tpl.Description, tpl.TemplateMarkdown, now, tpl.ID)
	if err == nil {
		tpl.UpdatedAt = now
	}
	return err
}

func (s *reportsStore) DeleteReportTemplate(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM report_templates WHERE id=?`, id)
	return err
}

func (s *reportsStore) EnsureDefaultReportTemplates(ctx context.Context) error {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM report_templates`)
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	now := time.Now().UTC()
	seed := []ReportTemplate{
		{
			Name: "Ежемесячный отчёт ИБ",
			Description: "Структурированный ежемесячный отчёт по ИБ",
			TemplateMarkdown: "## Резюме\n\n## Область отчёта\n\n## Наблюдения\n\n## Метрики\n\n## Риски и рекомендации\n",
		},
		{
			Name: "Отчёт руководству (кратко)",
			Description: "EXECUTIVE_RU: Короткий отчёт для руководства",
			TemplateMarkdown: "## Ключевые KPI\n\n## Основные риски\n\n## Графики\n\n## Рекомендации\n",
		},
		{
			Name: "Executive brief report",
			Description: "EXECUTIVE_EN: Short management report",
			TemplateMarkdown: "## Key KPIs\n\n## Top risks\n\n## Charts\n\n## Recommendations\n",
		},
		{
			Name: "Отчёт по контролям (квартал)",
			Description: "Отчёт по эффективности контролей за квартал",
			TemplateMarkdown: "## Обзор контролей\n\n## Выполнение и отклонения\n\n## План действий\n",
		},
	}
	if count > 0 {
		for _, tpl := range seed {
			if !strings.Contains(strings.ToUpper(tpl.Description), "EXECUTIVE_") {
				continue
			}
			if err := s.ensureTemplateByTag(ctx, tpl, now); err != nil {
				return err
			}
		}
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, tpl := range seed {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO report_templates(name, description, template_markdown, created_by, created_at, updated_at)
			VALUES(?,?,?,?,?,?)`, tpl.Name, tpl.Description, tpl.TemplateMarkdown, 0, now, now); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *reportsStore) ensureTemplateByTag(ctx context.Context, tpl ReportTemplate, now time.Time) error {
	tag := tpl.Description
	if idx := strings.Index(tag, ":"); idx >= 0 {
		tag = tag[:idx]
	}
	tag = strings.ToUpper(strings.TrimSpace(tag))
	if tag == "" {
		return nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM report_templates WHERE UPPER(description) LIKE ?`, "%"+tag+"%")
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO report_templates(name, description, template_markdown, created_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?)`, tpl.Name, tpl.Description, tpl.TemplateMarkdown, 0, now, now)
	return err
}

func (s *reportsStore) GetReportSettings(ctx context.Context) (*ReportSettings, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, default_classification, default_template_id, header_enabled, header_logo_path, header_title, watermark_threshold, updated_at
		FROM report_settings ORDER BY id LIMIT 1`)
	var settings ReportSettings
	var headerEnabled int
	var defaultTemplate sql.NullInt64
	if err := row.Scan(&settings.ID, &settings.DefaultClassification, &defaultTemplate, &headerEnabled, &settings.HeaderLogoPath, &settings.HeaderTitle, &settings.WatermarkThreshold, &settings.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			settings = defaultReportSettings()
			if _, err := s.insertReportSettings(ctx, &settings); err != nil {
				return nil, err
			}
			return &settings, nil
		}
		return nil, err
	}
	if defaultTemplate.Valid {
		settings.DefaultTemplateID = &defaultTemplate.Int64
	}
	settings.HeaderEnabled = headerEnabled == 1
	return &settings, nil
}

func (s *reportsStore) UpdateReportSettings(ctx context.Context, settings *ReportSettings) error {
	if settings == nil {
		return errors.New("missing settings")
	}
	now := time.Now().UTC()
	if settings.ID == 0 {
		settings.UpdatedAt = now
		_, err := s.insertReportSettings(ctx, settings)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE report_settings
		SET default_classification=?, default_template_id=?, header_enabled=?, header_logo_path=?, header_title=?, watermark_threshold=?, updated_at=?
		WHERE id=?`,
		settings.DefaultClassification, nullableID(settings.DefaultTemplateID), boolToInt(settings.HeaderEnabled), settings.HeaderLogoPath, settings.HeaderTitle, settings.WatermarkThreshold, now, settings.ID)
	if err == nil {
		settings.UpdatedAt = now
	}
	return err
}

func (s *reportsStore) insertReportSettings(ctx context.Context, settings *ReportSettings) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO report_settings(default_classification, default_template_id, header_enabled, header_logo_path, header_title, watermark_threshold, updated_at)
		VALUES(?,?,?,?,?,?,?)`,
		settings.DefaultClassification, nullableID(settings.DefaultTemplateID), boolToInt(settings.HeaderEnabled), settings.HeaderLogoPath, settings.HeaderTitle, settings.WatermarkThreshold, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	settings.ID = id
	settings.UpdatedAt = now
	return id, nil
}

func defaultReportSettings() ReportSettings {
	return ReportSettings{
		DefaultClassification: "INTERNAL",
		HeaderEnabled:         true,
		HeaderLogoPath:        "/gui/static/logo.png",
		HeaderTitle:           "Berkut Solutions: Security Control Center",
		WatermarkThreshold:    "",
		UpdatedAt:             time.Now().UTC(),
	}
}

func scanReportMeta(row interface {
	Scan(dest ...any) error
}) (*ReportMeta, error) {
	var meta ReportMeta
	var periodFrom, periodTo sql.NullTime
	var template sql.NullInt64
	if err := row.Scan(&meta.DocID, &periodFrom, &periodTo, &meta.Status, &template); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if periodFrom.Valid {
		meta.PeriodFrom = &periodFrom.Time
	}
	if periodTo.Valid {
		meta.PeriodTo = &periodTo.Time
	}
	if template.Valid {
		meta.TemplateID = &template.Int64
	}
	if strings.TrimSpace(meta.Status) == "" {
		meta.Status = "draft"
	}
	return &meta, nil
}

