package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Row is one event in a list.
type Row struct {
	ID       int64
	Hash     string
	Program  string
	Cmd      string
	ExitCode *int
	OutLen   int
	OutLines int
	ErrLen   int
	Trunc    bool
	Error    bool
	Surprise float64
	Sources  int
}

// Filter narrows an event list. Zero values mean "any".
type Filter struct {
	Q       string // full-text over command and output
	Program string
	Exit    string // "ok", "fail", "none", or a number
	Source  string
	Split   string
	Trunc   bool
	Stderr  bool
	Sort    string // "" (newest id), "surprise", "long"
	After   int64  // keyset cursor: id
	Limit   int
}

func (f Filter) where() (string, []any) {
	var w []string
	var args []any
	if f.Q != "" {
		w = append(w, `e.id IN (SELECT rowid FROM events_fts WHERE events_fts MATCH ?)`)
		args = append(args, ftsQuery(f.Q))
	}
	if f.Program != "" {
		w = append(w, `e.program = ?`)
		args = append(args, f.Program)
	}
	switch f.Exit {
	case "":
	case "ok":
		w = append(w, `e.exit_code = 0`)
	case "fail":
		w = append(w, `e.exit_code <> 0`)
	case "none":
		w = append(w, `e.exit_code IS NULL`)
	default:
		w = append(w, `e.exit_code = ?`)
		args = append(args, f.Exit)
	}
	if f.Source != "" || f.Split != "" {
		q := `e.id IN (SELECT event FROM sources WHERE 1=1`
		if f.Source != "" {
			q += ` AND source = ?`
			args = append(args, f.Source)
		}
		if f.Split != "" {
			q += ` AND split = ?`
			args = append(args, f.Split)
		}
		w = append(w, q+`)`)
	}
	if f.Trunc {
		w = append(w, `e.truncated = 1`)
	}
	if f.Stderr {
		w = append(w, `e.err_len > 0`)
	}
	if len(w) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w, " AND "), args
}

// ftsQuery quotes each word so user text can't use FTS syntax.
func ftsQuery(q string) string {
	var parts []string
	for _, t := range strings.Fields(q) {
		parts = append(parts, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " ")
}

const rowCols = `e.id, e.hash, e.program, e.cmd, e.exit_code, e.out_len, e.out_lines, e.err_len, e.truncated, e.error, COALESCE(e.surprise, 0), (SELECT COUNT(*) FROM sources s WHERE s.event = e.id)`

func scanRow(rows *sql.Rows) (Row, error) {
	var r Row
	var code sql.NullInt64
	err := rows.Scan(&r.ID, &r.Hash, &r.Program, &r.Cmd, &code, &r.OutLen, &r.OutLines, &r.ErrLen, &r.Trunc, &r.Error, &r.Surprise, &r.Sources)
	if code.Valid {
		c := int(code.Int64)
		r.ExitCode = &c
	}
	return r, err
}

// List returns a page of events plus whether more follow. Offset pagination for
// sorted views, keyset for the default.
func (s *Store) List(f Filter, offset int) ([]Row, bool, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where, args := f.where()
	order := ` ORDER BY e.id`
	switch f.Sort {
	case "surprise":
		order = ` ORDER BY e.surprise DESC, e.id`
	case "long":
		order = ` ORDER BY e.out_len DESC, e.id`
	}
	q := `SELECT ` + rowCols + ` FROM events e` + where + order + fmt.Sprintf(` LIMIT %d OFFSET %d`, f.Limit+1, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	more := len(out) > f.Limit
	if more {
		out = out[:f.Limit]
	}
	return out, more, rows.Err()
}

// Count returns how many events match.
func (s *Store) Count(f Filter) (int, error) {
	where, args := f.where()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM events e`+where, args...).Scan(&n)
	return n, err
}

// Lookup finds an event's row by hash (or unique prefix of at least 8).
func (s *Store) Lookup(hash string) (*Row, error) {
	if len(hash) < 8 {
		return nil, sql.ErrNoRows
	}
	rows, err := s.db.Query(`SELECT `+rowCols+` FROM events e WHERE e.hash >= ? AND e.hash < ? LIMIT 2`, hash, hash+"\xff")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if len(out) != 1 {
		return nil, sql.ErrNoRows
	}
	return &out[0], nil
}

// Sources lists where an event came from.
func (s *Store) Sources(id int64) ([]Source, error) {
	rows, err := s.db.Query(`SELECT source, unit, split, turn, call, meta FROM sources WHERE event = ? ORDER BY source, unit, turn, call LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var src Source
		var meta string
		if err := rows.Scan(&src.Source, &src.Unit, &src.Split, &src.Turn, &src.Call, &meta); err != nil {
			return nil, err
		}
		src.Meta = []byte(meta)
		out = append(out, src)
	}
	return out, rows.Err()
}

// Count of one stat field.
type Stat struct {
	Value string
	Count int
}

// Stats returns a field's base-rate table, most common first.
func (s *Store) Stats(field string, limit int) ([]Stat, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM stats WHERE field = ?`, field).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := `count DESC, value`
	if field == "out_len_log2" {
		order = `CAST(value AS INTEGER)`
	}
	rows, err := s.db.Query(`SELECT value, count FROM stats WHERE field = ? ORDER BY `+order+fmt.Sprintf(` LIMIT %d`, limit), field)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Stat
	for rows.Next() {
		var st Stat
		if err := rows.Scan(&st.Value, &st.Count); err != nil {
			return nil, 0, err
		}
		out = append(out, st)
	}
	return out, total, rows.Err()
}

// SourceCount is one row of the corpus overview.
type SourceCount struct {
	Source string
	Split  string
	Occ    int // occurrences (source rows)
	Events int // distinct events
	Units  int // trajectories or sessions
}

func (s *Store) SourceCounts() ([]SourceCount, error) {
	rows, err := s.db.Query(`SELECT source, split, COUNT(*), COUNT(DISTINCT event), COUNT(DISTINCT unit) FROM sources GROUP BY source, split ORDER BY source, split`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceCount
	for rows.Next() {
		var c SourceCount
		if err := rows.Scan(&c.Source, &c.Split, &c.Occ, &c.Events, &c.Units); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Totals for the overview.
type Totals struct {
	Events, Occurrences, Sources, Units, Leaked int
}

func (s *Store) Totals() (Totals, error) {
	var t Totals
	err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM events), (SELECT COUNT(*) FROM sources), (SELECT COUNT(DISTINCT source) FROM sources), (SELECT COUNT(DISTINCT unit) FROM sources),
		(SELECT COUNT(*) FROM (SELECT event FROM sources WHERE split IN ('train','test') GROUP BY event HAVING COUNT(DISTINCT split) > 1))`).Scan(&t.Events, &t.Occurrences, &t.Sources, &t.Units, &t.Leaked)
	return t, err
}

// Sample returns random event texts for building neighbour indexes.
func (s *Store) Programs() ([]string, error) {
	rows, err := s.db.Query(`SELECT value FROM stats WHERE field = 'program' ORDER BY count DESC LIMIT 300`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		out = append(out, v)
	}
	return out, rows.Err()
}

// SampleTexts returns up to n distinct commands, a deterministic sample
// (ordered by content hash, which is effectively random), for corpus statistics.
func (s *Store) SampleTexts(field string, n int) ([]string, error) {
	if field != "command" {
		return nil, fmt.Errorf("unknown field %q", field)
	}
	rows, err := s.db.Query(`SELECT cmd FROM events WHERE length(cmd) BETWEEN 2 AND 300 GROUP BY cmd ORDER BY MIN(hash) LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		out = append(out, t)
	}
	return out, rows.Err()
}

// FieldTexts returns up to limit distinct values of a text field (command or
// first output line) with their event hashes, for the similarity index (08/K1d).
func (s *Store) FieldTexts(field string, limit int) ([]string, []string, error) {
	var q string
	switch field {
	case "command":
		q = `SELECT hash, cmd FROM events WHERE length(cmd) BETWEEN 2 AND 300 GROUP BY cmd ORDER BY MIN(id) LIMIT ?`
	default:
		return nil, nil, fmt.Errorf("unknown field %q", field)
	}
	rows, err := s.db.Query(q, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var hs, ts []string
	for rows.Next() {
		var h, t string
		rows.Scan(&h, &t)
		hs = append(hs, h)
		ts = append(ts, t)
	}
	return hs, ts, rows.Err()
}
