package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
)

const postgresDriverName = "pgx-rewrite"

func init() {
	sql.Register(postgresDriverName, rewriteDriver{base: stdlib.GetDefaultDriver()})
}

type rewriteDriver struct {
	base driver.Driver
}

func (d rewriteDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &rewriteConn{Conn: c}, nil
}

type rewriteConn struct {
	driver.Conn
}

func (c *rewriteConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(rewriteSQL(query))
}

func (c *rewriteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, rewriteSQL(query))
	}
	return c.Prepare(query)
}

func (c *rewriteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := rewriteSQL(query)
	isInsert := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(q)), "INSERT ")
	if ex, ok := c.Conn.(driver.ExecerContext); ok {
		res, err := ex.ExecContext(ctx, q, args)
		if err != nil {
			return nil, err
		}
		return c.wrapResultWithLastInsertID(ctx, q, res, args, isInsert)
	}
	if ex, ok := c.Conn.(driver.Execer); ok {
		vals, err := namedToValues(args)
		if err != nil {
			return nil, err
		}
		res, err := ex.Exec(q, vals)
		if err != nil {
			return nil, err
		}
		return c.wrapResultWithLastInsertID(ctx, q, res, args, isInsert)
	}
	return nil, driver.ErrSkip
}

func (c *rewriteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := rewriteSQL(query)
	if qx, ok := c.Conn.(driver.QueryerContext); ok {
		return qx.QueryContext(ctx, q, args)
	}
	if qx, ok := c.Conn.(driver.Queryer); ok {
		vals, err := namedToValues(args)
		if err != nil {
			return nil, err
		}
		return qx.Query(q, vals)
	}
	return nil, driver.ErrSkip
}

func (c *rewriteConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	if opts.ReadOnly {
		return nil, errors.New("driver does not support read-only transactions")
	}
	return c.Conn.Begin()
}

func namedToValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, 0, len(args))
	for _, a := range args {
		if a.Name != "" {
			return nil, errors.New("named parameters are not supported")
		}
		values = append(values, a.Value)
	}
	return values, nil
}

type rewriteResult struct {
	base         driver.Result
	lastInsertID int64
	hasLastID    bool
}

func (r rewriteResult) LastInsertId() (int64, error) {
	if !r.hasLastID {
		return 0, nil
	}
	return r.lastInsertID, nil
}

func (r rewriteResult) RowsAffected() (int64, error) {
	if r.base == nil {
		return 0, nil
	}
	return r.base.RowsAffected()
}

func (c *rewriteConn) wrapResultWithLastInsertID(ctx context.Context, query string, res driver.Result, args []driver.NamedValue, isInsert bool) (driver.Result, error) {
	if !isInsert {
		return rewriteResult{base: res}, nil
	}
	lastID, ok := c.resolveLastInsertID(ctx, query)
	if ok {
		return rewriteResult{base: res, lastInsertID: lastID, hasLastID: true}, nil
	}
	_ = args
	return rewriteResult{base: res}, nil
}

var reInsertIntoTable = regexp.MustCompile(`(?is)^\s*insert\s+into\s+("?[\w.]+"?)`)

func (c *rewriteConn) resolveLastInsertID(ctx context.Context, query string) (int64, bool) {
	table := extractInsertTable(query)
	if table == "" {
		return 0, false
	}
	const q = `SELECT currval(pg_get_serial_sequence($1, 'id'))`
	args := []driver.NamedValue{{Ordinal: 1, Value: table}}
	if qx, ok := c.Conn.(driver.QueryerContext); ok {
		rows, err := qx.QueryContext(ctx, q, args)
		if err != nil {
			return 0, false
		}
		defer rows.Close()
		cols := rows.Columns()
		if len(cols) < 1 {
			return 0, false
		}
		dest := make([]driver.Value, 1)
		if err := rows.Next(dest); err != nil {
			return 0, false
		}
		if id, ok := asInt64(dest[0]); ok {
			return id, true
		}
		return 0, false
	}
	return 0, false
}

func extractInsertTable(query string) string {
	m := reInsertIntoTable.FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int32:
		return int64(t), true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case []byte:
		n, err := strconv.ParseInt(string(t), 10, 64)
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

var reInsertOrIgnore = regexp.MustCompile(`(?is)^\s*insert\s+or\s+ignore\s+into\s+`)

func rewriteSQL(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return query
	}
	rewritten := query
	upper := strings.ToUpper(rewritten)
	if strings.Contains(upper, "DOCS_FTS MATCH ?") {
		rewritten = strings.ReplaceAll(
			rewritten,
			"docs_fts MATCH ?",
			"to_tsvector('simple', docs_fts.content) @@ plainto_tsquery('simple', ?)",
		)
	}
	if reInsertOrIgnore.MatchString(rewritten) {
		rewritten = reInsertOrIgnore.ReplaceAllString(rewritten, "INSERT INTO ")
		rewritten = strings.TrimSpace(rewritten)
		rewritten = strings.TrimSuffix(rewritten, ";")
		rewritten += " ON CONFLICT DO NOTHING"
	}
	return questionToDollar(rewritten)
}

func questionToDollar(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16)
	arg := 1
	inSingle := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			if inSingle {
				if i+1 < len(query) && query[i+1] == '\'' {
					b.WriteByte(ch)
					b.WriteByte(query[i+1])
					i++
					continue
				}
				inSingle = false
				b.WriteByte(ch)
				continue
			}
			inSingle = true
			b.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingle {
			b.WriteByte('$')
			b.WriteString(intToString(arg))
			arg++
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func intToString(n int) string {
	return strconv.Itoa(n)
}
