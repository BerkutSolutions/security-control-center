package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Control struct {
	ID              int64      `json:"id"`
	Code            string     `json:"code"`
	Title           string     `json:"title"`
	DescriptionMD   string     `json:"description_md"`
	ControlType     string     `json:"control_type"`
	Domain          string     `json:"domain"`
	OwnerUserID     *int64     `json:"owner_user_id,omitempty"`
	ReviewFrequency string     `json:"review_frequency"`
	Status          string     `json:"status"`
	RiskLevel       string     `json:"risk_level"`
	Tags            []string   `json:"tags"`
	CreatedBy       int64      `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	IsActive        bool       `json:"is_active"`
	LastCheckAt     *time.Time `json:"last_check_at,omitempty"`
	LastCheckResult string     `json:"last_check_result,omitempty"`
}

type ControlCheck struct {
	ID            int64     `json:"id"`
	ControlID     int64     `json:"control_id"`
	CheckedAt     time.Time `json:"checked_at"`
	CheckedBy     int64     `json:"checked_by"`
	Result        string    `json:"result"`
	NotesMD       string    `json:"notes_md"`
	EvidenceLinks []string  `json:"evidence_links"`
	CreatedAt     time.Time `json:"created_at"`
	ControlCode   string    `json:"control_code,omitempty"`
	ControlTitle  string    `json:"control_title,omitempty"`
}

type ControlViolation struct {
	ID           int64     `json:"id"`
	ControlID    int64     `json:"control_id"`
	IncidentID   *int64    `json:"incident_id,omitempty"`
	HappenedAt   time.Time `json:"happened_at"`
	Severity     string    `json:"severity"`
	Summary      string    `json:"summary"`
	ImpactMD     string    `json:"impact_md"`
	CreatedBy    int64     `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	IsAuto       bool      `json:"is_auto"`
	IsActive     bool      `json:"is_active"`
	ControlCode  string    `json:"control_code,omitempty"`
	ControlTitle string    `json:"control_title,omitempty"`
}

type ControlFramework struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	IsActive bool   `json:"is_active"`
}

type ControlFrameworkItem struct {
	ID           int64  `json:"id"`
	FrameworkID  int64  `json:"framework_id"`
	Code         string `json:"code"`
	Title        string `json:"title"`
	DescriptionMD string `json:"description_md"`
}

type ControlFrameworkMap struct {
	ID              int64 `json:"id"`
	FrameworkItemID int64 `json:"framework_item_id"`
	ControlID       int64 `json:"control_id"`
}

type ControlFilter struct {
	Search      string
	Status      string
	RiskLevel   string
	Domain      string
	OwnerUserID *int64
	Tag         string
	Active      *bool
}

type ControlCheckFilter struct {
	ControlID int64
	Result    string
	CheckedBy *int64
	DateFrom  *time.Time
	DateTo    *time.Time
}

type ControlViolationFilter struct {
	ControlID  int64
	IncidentID *int64
	Severity   string
	DateFrom   *time.Time
	DateTo     *time.Time
	Active     *bool
}

type ControlsStore interface {
	ListControlTypes(ctx context.Context) ([]ControlType, error)
	GetControlTypeByID(ctx context.Context, id int64) (*ControlType, error)
	GetControlTypeByName(ctx context.Context, name string) (*ControlType, error)
	CreateControlType(ctx context.Context, name string, isBuiltin bool) (*ControlType, error)
	DeleteControlType(ctx context.Context, id int64) error

	CreateControl(ctx context.Context, c *Control) (int64, error)
	UpdateControl(ctx context.Context, c *Control) error
	SoftDeleteControl(ctx context.Context, id int64) error
	GetControl(ctx context.Context, id int64) (*Control, error)
	ListControls(ctx context.Context, filter ControlFilter) ([]Control, error)

	CreateControlCheck(ctx context.Context, c *ControlCheck) (int64, error)
	GetControlCheck(ctx context.Context, id int64) (*ControlCheck, error)
	ListControlChecks(ctx context.Context, controlID int64) ([]ControlCheck, error)
	ListChecks(ctx context.Context, filter ControlCheckFilter) ([]ControlCheck, error)
	DeleteControlCheck(ctx context.Context, id int64) error

	CreateControlViolation(ctx context.Context, v *ControlViolation) (int64, error)
	GetControlViolationByLink(ctx context.Context, controlID int64, incidentID int64) (*ControlViolation, error)
	UpdateControlViolationAuto(ctx context.Context, v *ControlViolation) error
	GetControlViolation(ctx context.Context, id int64) (*ControlViolation, error)
	ListControlViolations(ctx context.Context, controlID int64) ([]ControlViolation, error)
	ListViolations(ctx context.Context, filter ControlViolationFilter) ([]ControlViolation, error)
	SoftDeleteControlViolation(ctx context.Context, id int64) error

	CreateFramework(ctx context.Context, f *ControlFramework) (int64, error)
	ListFrameworks(ctx context.Context) ([]ControlFramework, error)
	GetFramework(ctx context.Context, id int64) (*ControlFramework, error)
	CreateFrameworkItem(ctx context.Context, item *ControlFrameworkItem) (int64, error)
	GetFrameworkItem(ctx context.Context, id int64) (*ControlFrameworkItem, error)
	ListFrameworkItems(ctx context.Context, frameworkID int64) ([]ControlFrameworkItem, error)
	AddFrameworkMap(ctx context.Context, m *ControlFrameworkMap) (int64, error)
	ListFrameworkMap(ctx context.Context, frameworkID int64) ([]ControlFrameworkMap, error)

	AddControlComment(ctx context.Context, comment *ControlComment) (int64, error)
	ListControlComments(ctx context.Context, controlID int64) ([]ControlComment, error)
	GetControlComment(ctx context.Context, commentID int64) (*ControlComment, error)
	UpdateControlComment(ctx context.Context, comment *ControlComment) error
	DeleteControlComment(ctx context.Context, commentID int64) error
}

type controlsStore struct {
	db *sql.DB
}

func NewControlsStore(db *sql.DB) ControlsStore {
	return &controlsStore{db: db}
}

func (s *controlsStore) CreateControl(ctx context.Context, c *Control) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO controls(code, title, description_md, control_type, domain, owner_user_id, review_frequency, status, risk_level, tags_json, created_by, created_at, updated_at, is_active)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.Code, c.Title, c.DescriptionMD, c.ControlType, c.Domain, nullableID(c.OwnerUserID), c.ReviewFrequency, c.Status, c.RiskLevel, tagsToJSON(normalizeTags(c.Tags)), c.CreatedBy, now, now, boolToInt(c.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	c.CreatedAt = now
	c.UpdatedAt = now
	return id, nil
}

func (s *controlsStore) UpdateControl(ctx context.Context, c *Control) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE controls SET code=?, title=?, description_md=?, control_type=?, domain=?, owner_user_id=?, review_frequency=?, status=?, risk_level=?, tags_json=?, updated_at=?, is_active=?
		WHERE id=?`,
		c.Code, c.Title, c.DescriptionMD, c.ControlType, c.Domain, nullableID(c.OwnerUserID), c.ReviewFrequency, c.Status, c.RiskLevel, tagsToJSON(normalizeTags(c.Tags)), time.Now().UTC(), boolToInt(c.IsActive), c.ID)
	return err
}

func (s *controlsStore) SoftDeleteControl(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE controls SET is_active=0, updated_at=? WHERE id=?`, time.Now().UTC(), id)
	return err
}

func (s *controlsStore) GetControl(ctx context.Context, id int64) (*Control, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.code, c.title, c.description_md, c.control_type, c.domain, c.owner_user_id, c.review_frequency, c.status, c.risk_level, c.tags_json, c.created_by, c.created_at, c.updated_at, c.is_active,
		       (SELECT checked_at FROM control_checks cc WHERE cc.control_id=c.id ORDER BY checked_at DESC LIMIT 1) AS last_check_at,
		       (SELECT result FROM control_checks cc WHERE cc.control_id=c.id ORDER BY checked_at DESC LIMIT 1) AS last_check_result
		FROM controls c WHERE c.id=?`, id)
	return scanControl(row)
}

func (s *controlsStore) ListControls(ctx context.Context, filter ControlFilter) ([]Control, error) {
	clauses := []string{}
	args := []any{}
	if filter.Search != "" {
		clauses = append(clauses, "(LOWER(code) LIKE ? OR LOWER(title) LIKE ? OR LOWER(description_md) LIKE ?)")
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status=?")
		args = append(args, filter.Status)
	}
	if filter.RiskLevel != "" {
		clauses = append(clauses, "risk_level=?")
		args = append(args, filter.RiskLevel)
	}
	if filter.Domain != "" {
		clauses = append(clauses, "domain=?")
		args = append(args, filter.Domain)
	}
	if filter.OwnerUserID != nil {
		clauses = append(clauses, "owner_user_id=?")
		args = append(args, *filter.OwnerUserID)
	}
	if filter.Tag != "" {
		tag := strings.ToUpper(strings.TrimSpace(filter.Tag))
		if tag != "" {
			clauses = append(clauses, "tags_json LIKE ?")
			args = append(args, "%"+tag+"%")
		}
	}
	if filter.Active == nil {
		clauses = append(clauses, "is_active=1")
	} else {
		clauses = append(clauses, "is_active=?")
		args = append(args, boolToInt(*filter.Active))
	}
	query := `
		SELECT c.id, c.code, c.title, c.description_md, c.control_type, c.domain, c.owner_user_id, c.review_frequency, c.status, c.risk_level, c.tags_json, c.created_by, c.created_at, c.updated_at, c.is_active,
		       (SELECT checked_at FROM control_checks cc WHERE cc.control_id=c.id ORDER BY checked_at DESC LIMIT 1) AS last_check_at,
		       (SELECT result FROM control_checks cc WHERE cc.control_id=c.id ORDER BY checked_at DESC LIMIT 1) AS last_check_result
		FROM controls c`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Control
	for rows.Next() {
		item, err := scanControlRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *controlsStore) CreateControlCheck(ctx context.Context, c *ControlCheck) (int64, error) {
	now := time.Now().UTC()
	linksJSON, _ := json.Marshal(c.EvidenceLinks)
	var checkedBy *int64
	if c.CheckedBy > 0 {
		checkedBy = &c.CheckedBy
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_checks(control_id, checked_at, checked_by, result, notes_md, evidence_links_json, created_at)
		VALUES(?,?,?,?,?,?,?)`,
		c.ControlID, c.CheckedAt, nullableID(checkedBy), c.Result, c.NotesMD, string(linksJSON), now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	c.CreatedAt = now
	return id, nil
}

func (s *controlsStore) GetControlCheck(ctx context.Context, id int64) (*ControlCheck, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT ch.id, ch.control_id, ch.checked_at, ch.checked_by, ch.result, ch.notes_md, ch.evidence_links_json, ch.created_at, c.code, c.title
		FROM control_checks ch
		INNER JOIN controls c ON c.id=ch.control_id
		WHERE ch.id=?`, id)
	return scanControlCheck(row)
}

func (s *controlsStore) ListControlChecks(ctx context.Context, controlID int64) ([]ControlCheck, error) {
	return s.ListChecks(ctx, ControlCheckFilter{ControlID: controlID})
}

func (s *controlsStore) ListChecks(ctx context.Context, filter ControlCheckFilter) ([]ControlCheck, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if filter.ControlID > 0 {
		clauses = append(clauses, "ch.control_id=?")
		args = append(args, filter.ControlID)
	}
	if filter.Result != "" {
		clauses = append(clauses, "ch.result=?")
		args = append(args, filter.Result)
	}
	if filter.CheckedBy != nil {
		clauses = append(clauses, "ch.checked_by=?")
		args = append(args, *filter.CheckedBy)
	}
	if filter.DateFrom != nil {
		clauses = append(clauses, "ch.checked_at>=?")
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		clauses = append(clauses, "ch.checked_at<=?")
		args = append(args, *filter.DateTo)
	}
	query := `
		SELECT ch.id, ch.control_id, ch.checked_at, ch.checked_by, ch.result, ch.notes_md, ch.evidence_links_json, ch.created_at, c.code, c.title
		FROM control_checks ch
		INNER JOIN controls c ON c.id=ch.control_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ch.checked_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlCheck
	for rows.Next() {
		item, err := scanControlCheckRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *controlsStore) DeleteControlCheck(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM control_checks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("not found")
	}
	return nil
}

func (s *controlsStore) CreateControlViolation(ctx context.Context, v *ControlViolation) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_violations(control_id, incident_id, happened_at, severity, summary, impact_md, created_by, created_at, is_auto, is_active)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		v.ControlID, nullableID(v.IncidentID), v.HappenedAt, v.Severity, v.Summary, v.ImpactMD, v.CreatedBy, now, boolToInt(v.IsAuto), boolToInt(v.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	v.ID = id
	v.CreatedAt = now
	return id, nil
}

func (s *controlsStore) GetControlViolationByLink(ctx context.Context, controlID int64, incidentID int64) (*ControlViolation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT v.id, v.control_id, v.incident_id, v.happened_at, v.severity, v.summary, v.impact_md, v.created_by, v.created_at, v.is_auto, v.is_active, c.code, c.title
		FROM control_violations v
		INNER JOIN controls c ON c.id=v.control_id
		WHERE v.control_id=? AND v.incident_id=?`, controlID, incidentID)
	return scanControlViolation(row)
}

func (s *controlsStore) UpdateControlViolationAuto(ctx context.Context, v *ControlViolation) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE control_violations
		SET happened_at=?, severity=?, summary=?, impact_md=?, is_auto=?, is_active=?
		WHERE id=?`,
		v.HappenedAt, v.Severity, v.Summary, v.ImpactMD, boolToInt(v.IsAuto), boolToInt(v.IsActive), v.ID)
	return err
}

func (s *controlsStore) GetControlViolation(ctx context.Context, id int64) (*ControlViolation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT v.id, v.control_id, v.incident_id, v.happened_at, v.severity, v.summary, v.impact_md, v.created_by, v.created_at, v.is_auto, v.is_active, c.code, c.title
		FROM control_violations v
		INNER JOIN controls c ON c.id=v.control_id
		WHERE v.id=?`, id)
	return scanControlViolation(row)
}

func (s *controlsStore) ListControlViolations(ctx context.Context, controlID int64) ([]ControlViolation, error) {
	return s.ListViolations(ctx, ControlViolationFilter{ControlID: controlID})
}

func (s *controlsStore) ListViolations(ctx context.Context, filter ControlViolationFilter) ([]ControlViolation, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if filter.ControlID > 0 {
		clauses = append(clauses, "v.control_id=?")
		args = append(args, filter.ControlID)
	}
	if filter.IncidentID != nil {
		clauses = append(clauses, "v.incident_id=?")
		args = append(args, *filter.IncidentID)
	}
	if filter.Severity != "" {
		clauses = append(clauses, "v.severity=?")
		args = append(args, filter.Severity)
	}
	if filter.DateFrom != nil {
		clauses = append(clauses, "v.happened_at>=?")
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		clauses = append(clauses, "v.happened_at<=?")
		args = append(args, *filter.DateTo)
	}
	if filter.Active == nil {
		clauses = append(clauses, "v.is_active=1")
	} else {
		clauses = append(clauses, "v.is_active=?")
		args = append(args, boolToInt(*filter.Active))
	}
	query := `
		SELECT v.id, v.control_id, v.incident_id, v.happened_at, v.severity, v.summary, v.impact_md, v.created_by, v.created_at, v.is_auto, v.is_active, c.code, c.title
		FROM control_violations v
		INNER JOIN controls c ON c.id=v.control_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY v.happened_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlViolation
	for rows.Next() {
		item, err := scanControlViolationRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *controlsStore) SoftDeleteControlViolation(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE control_violations SET is_active=0 WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("not found")
	}
	return nil
}

func (s *controlsStore) CreateFramework(ctx context.Context, f *ControlFramework) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_frameworks(name, version, is_active) VALUES(?,?,?)`,
		f.Name, f.Version, boolToInt(f.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	f.ID = id
	return id, nil
}

func (s *controlsStore) ListFrameworks(ctx context.Context) ([]ControlFramework, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, version, is_active FROM control_frameworks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlFramework
	for rows.Next() {
		var f ControlFramework
		var active int
		if err := rows.Scan(&f.ID, &f.Name, &f.Version, &active); err != nil {
			return nil, err
		}
		f.IsActive = active == 1
		res = append(res, f)
	}
	return res, rows.Err()
}

func (s *controlsStore) GetFramework(ctx context.Context, id int64) (*ControlFramework, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, version, is_active FROM control_frameworks WHERE id=?`, id)
	var f ControlFramework
	var active int
	if err := row.Scan(&f.ID, &f.Name, &f.Version, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	f.IsActive = active == 1
	return &f, nil
}

func (s *controlsStore) CreateFrameworkItem(ctx context.Context, item *ControlFrameworkItem) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_framework_items(framework_id, code, title, description_md)
		VALUES(?,?,?,?)`,
		item.FrameworkID, item.Code, item.Title, item.DescriptionMD)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	item.ID = id
	return id, nil
}

func (s *controlsStore) GetFrameworkItem(ctx context.Context, id int64) (*ControlFrameworkItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, framework_id, code, title, description_md
		FROM control_framework_items WHERE id=?`, id)
	var item ControlFrameworkItem
	if err := row.Scan(&item.ID, &item.FrameworkID, &item.Code, &item.Title, &item.DescriptionMD); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *controlsStore) ListFrameworkItems(ctx context.Context, frameworkID int64) ([]ControlFrameworkItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, framework_id, code, title, description_md
		FROM control_framework_items WHERE framework_id=? ORDER BY code`, frameworkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlFrameworkItem
	for rows.Next() {
		var item ControlFrameworkItem
		if err := rows.Scan(&item.ID, &item.FrameworkID, &item.Code, &item.Title, &item.DescriptionMD); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *controlsStore) AddFrameworkMap(ctx context.Context, m *ControlFrameworkMap) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_framework_map(framework_item_id, control_id)
		VALUES(?,?)`, m.FrameworkItemID, m.ControlID)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	return id, nil
}

func (s *controlsStore) ListFrameworkMap(ctx context.Context, frameworkID int64) ([]ControlFrameworkMap, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.framework_item_id, m.control_id
		FROM control_framework_map m
		INNER JOIN control_framework_items i ON i.id=m.framework_item_id
		WHERE i.framework_id=?`, frameworkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlFrameworkMap
	for rows.Next() {
		var m ControlFrameworkMap
		if err := rows.Scan(&m.ID, &m.FrameworkItemID, &m.ControlID); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

func scanControl(row interface {
	Scan(dest ...any) error
}) (*Control, error) {
	var c Control
	var tagsRaw string
	var owner sql.NullInt64
	var active int
	var lastCheckAt sql.NullTime
	var lastCheckResult sql.NullString
	if err := row.Scan(&c.ID, &c.Code, &c.Title, &c.DescriptionMD, &c.ControlType, &c.Domain, &owner, &c.ReviewFrequency, &c.Status, &c.RiskLevel, &tagsRaw, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &active, &lastCheckAt, &lastCheckResult); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if owner.Valid {
		c.OwnerUserID = &owner.Int64
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &c.Tags)
	}
	c.IsActive = active == 1
	if lastCheckAt.Valid {
		c.LastCheckAt = &lastCheckAt.Time
	}
	if lastCheckResult.Valid {
		c.LastCheckResult = lastCheckResult.String
	}
	return &c, nil
}

func scanControlRow(rows *sql.Rows) (Control, error) {
	var c Control
	var tagsRaw string
	var owner sql.NullInt64
	var active int
	var lastCheckAt sql.NullTime
	var lastCheckResult sql.NullString
	if err := rows.Scan(&c.ID, &c.Code, &c.Title, &c.DescriptionMD, &c.ControlType, &c.Domain, &owner, &c.ReviewFrequency, &c.Status, &c.RiskLevel, &tagsRaw, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &active, &lastCheckAt, &lastCheckResult); err != nil {
		return c, err
	}
	if owner.Valid {
		c.OwnerUserID = &owner.Int64
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &c.Tags)
	}
	c.IsActive = active == 1
	if lastCheckAt.Valid {
		c.LastCheckAt = &lastCheckAt.Time
	}
	if lastCheckResult.Valid {
		c.LastCheckResult = lastCheckResult.String
	}
	return c, nil
}

func scanControlCheck(row interface {
	Scan(dest ...any) error
}) (*ControlCheck, error) {
	var c ControlCheck
	var evidenceRaw string
	var checkedBy sql.NullInt64
	if err := row.Scan(&c.ID, &c.ControlID, &c.CheckedAt, &checkedBy, &c.Result, &c.NotesMD, &evidenceRaw, &c.CreatedAt, &c.ControlCode, &c.ControlTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if checkedBy.Valid {
		c.CheckedBy = checkedBy.Int64
	}
	if evidenceRaw != "" {
		_ = json.Unmarshal([]byte(evidenceRaw), &c.EvidenceLinks)
	}
	return &c, nil
}

func scanControlCheckRow(rows *sql.Rows) (ControlCheck, error) {
	var c ControlCheck
	var evidenceRaw string
	var checkedBy sql.NullInt64
	if err := rows.Scan(&c.ID, &c.ControlID, &c.CheckedAt, &checkedBy, &c.Result, &c.NotesMD, &evidenceRaw, &c.CreatedAt, &c.ControlCode, &c.ControlTitle); err != nil {
		return c, err
	}
	if checkedBy.Valid {
		c.CheckedBy = checkedBy.Int64
	}
	if evidenceRaw != "" {
		_ = json.Unmarshal([]byte(evidenceRaw), &c.EvidenceLinks)
	}
	return c, nil
}

func scanControlViolation(row interface {
	Scan(dest ...any) error
}) (*ControlViolation, error) {
	var v ControlViolation
	var incident sql.NullInt64
	var isAuto int
	var isActive int
	if err := row.Scan(&v.ID, &v.ControlID, &incident, &v.HappenedAt, &v.Severity, &v.Summary, &v.ImpactMD, &v.CreatedBy, &v.CreatedAt, &isAuto, &isActive, &v.ControlCode, &v.ControlTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if incident.Valid {
		v.IncidentID = &incident.Int64
	}
	v.IsAuto = isAuto == 1
	v.IsActive = isActive == 1
	return &v, nil
}

func scanControlViolationRow(rows *sql.Rows) (ControlViolation, error) {
	var v ControlViolation
	var incident sql.NullInt64
	var isAuto int
	var isActive int
	if err := rows.Scan(&v.ID, &v.ControlID, &incident, &v.HappenedAt, &v.Severity, &v.Summary, &v.ImpactMD, &v.CreatedBy, &v.CreatedAt, &isAuto, &isActive, &v.ControlCode, &v.ControlTitle); err != nil {
		return v, err
	}
	if incident.Valid {
		v.IncidentID = &incident.Int64
	}
	v.IsAuto = isAuto == 1
	v.IsActive = isActive == 1
	return v, nil
}
