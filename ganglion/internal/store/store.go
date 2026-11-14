// Package store keeps events as content-addressed blobs with a SQLite index.
//
// Layout under the data directory:
//
//	index.db   browsing index, sources, stats. Rebuildable from blobs + import logs.
//	blobs.db   blobs under SmallBlob bytes, keyed by hash (08/E1a).
//	blobs/     larger blobs as files, blobs/ab/<hash>.
//
// An event's ID is the SHA-256 (hex) of its canonical JSON encoding.
package store

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// SmallBlob is the size under which a blob lives in blobs.db instead of a file.
const SmallBlob = 4096

// Event is the command_execution schema (06/W1), in canonical field order.
type Event struct {
	Type       string  `json:"type"`
	Command    string  `json:"command"`
	Cwd        *string `json:"cwd"`
	ExitCode   *int    `json:"exit_code"`
	Output     *string `json:"output"`
	Stdout     *string `json:"stdout"`
	Stderr     *string `json:"stderr"`
	Truncated  bool    `json:"truncated"`
	Error      *string `json:"error"`
	DurationMs *int64  `json:"duration_ms"`
}

// Source says where one occurrence of an event came from.
type Source struct {
	Source   string          `json:"source"`
	Unit     string          `json:"unit"`
	SplitKey string          `json:"split_key"`
	Turn     int             `json:"turn"`
	Call     int             `json:"call"`
	Meta     json.RawMessage `json:"meta"`
	Split    string          `json:"split,omitempty"`
}

type Store struct {
	dir   string
	db    *sql.DB
	blobs *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY,
	hash TEXT NOT NULL UNIQUE,
	program TEXT NOT NULL,
	cmd TEXT NOT NULL,
	exit_code INTEGER,
	out_len INTEGER NOT NULL,
	out_lines INTEGER NOT NULL,
	err_len INTEGER NOT NULL,
	truncated INTEGER NOT NULL,
	error INTEGER NOT NULL,
	surprise REAL
);
CREATE INDEX IF NOT EXISTS events_program ON events(program);
CREATE INDEX IF NOT EXISTS events_exit ON events(exit_code);
CREATE TABLE IF NOT EXISTS sources (
	event INTEGER NOT NULL REFERENCES events(id),
	source TEXT NOT NULL,
	unit TEXT NOT NULL,
	split TEXT NOT NULL,
	turn INTEGER NOT NULL,
	call INTEGER NOT NULL,
	meta TEXT NOT NULL,
	UNIQUE(source, unit, turn, call)
);
CREATE INDEX IF NOT EXISTS sources_event ON sources(event);
CREATE INDEX IF NOT EXISTS sources_source ON sources(source, split);
CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(cmd, out, content='', tokenize='unicode61');
CREATE TABLE IF NOT EXISTS stats (field TEXT NOT NULL, value TEXT NOT NULL, count INTEGER NOT NULL, PRIMARY KEY(field, value));
`

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o755); err != nil {
		return nil, err
	}
	open := func(name string) (*sql.DB, error) {
		db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, name)+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(4)
		return db, nil
	}
	db, err := open("index.db")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	blobs, err := open("blobs.db")
	if err != nil {
		return nil, err
	}
	if _, err := blobs.Exec(`CREATE TABLE IF NOT EXISTS blobs (hash TEXT PRIMARY KEY, data BLOB NOT NULL) WITHOUT ROWID`); err != nil {
		return nil, err
	}
	return &Store{dir: dir, db: db, blobs: blobs}, nil
}

func (s *Store) Close() { s.db.Close(); s.blobs.Close() }

// Canonical returns the canonical bytes and hash of an event.
func Canonical(ev *Event) ([]byte, string, error) {
	b, err := json.Marshal(ev)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.dir, "blobs", hash[:2], hash)
}

func (s *Store) putBlob(tx *sql.Tx, hash string, data []byte) error {
	if len(data) < SmallBlob {
		_, err := tx.Exec(`INSERT OR IGNORE INTO blobs(hash, data) VALUES(?, ?)`, hash, data)
		return err
	}
	p := s.blobPath(hash)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Blob returns a blob's bytes.
func (s *Store) Blob(hash string) ([]byte, error) {
	var data []byte
	err := s.blobs.QueryRow(`SELECT data FROM blobs WHERE hash = ?`, hash).Scan(&data)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return os.ReadFile(s.blobPath(hash))
}

// Event loads an event by hash.
func (s *Store) Event(hash string) (*Event, error) {
	b, err := s.Blob(hash)
	if err != nil {
		return nil, err
	}
	var ev Event
	return &ev, json.Unmarshal(b, &ev)
}

// splitOf assigns held-out membership by split key (06/G7): 1 in 10 keys is "test".
// Local sources are their own split (08/D40).
func splitOf(src Source) string {
	if strings.HasPrefix(src.Source, "local/") {
		return "local"
	}
	sum := sha256.Sum256([]byte(src.SplitKey))
	if sum[0]%10 == 0 {
		return "test"
	}
	return "train"
}

// Program is a software-derived field: the first program run, skipping cd,
// environment assignments and common wrappers. A display and filter aid only.
func Program(cmd string) string {
	segs := strings.FieldsFunc(cmd, func(r rune) bool { return r == '&' || r == ';' || r == '|' || r == '\n' || r == '(' || r == ')' })
	for _, seg := range segs {
		toks := strings.Fields(seg)
		for len(toks) > 0 {
			t := toks[0]
			if strings.Contains(t, "=") && !strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "=") {
				toks = toks[1:]
				continue
			}
			if t == "sudo" || t == "timeout" || t == "env" || t == "time" || t == "nohup" || t == "exec" {
				toks = toks[1:]
				if len(toks) > 0 && t == "timeout" {
					toks = toks[1:]
				}
				continue
			}
			break
		}
		if len(toks) == 0 || toks[0] == "cd" || toks[0] == "set" || toks[0] == "export" {
			continue
		}
		p := strings.Trim(toks[0], `"'`)
		if i := strings.LastIndexByte(p, '/'); i >= 0 && i < len(p)-1 {
			p = p[i+1:]
		}
		if p == "" {
			continue
		}
		if len(p) > 40 {
			p = p[:40]
		}
		return p
	}
	return "(none)"
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// MainText is the event's primary output text: combined output, else stdout.
func (ev *Event) MainText() string {
	if ev.Output != nil {
		return *ev.Output
	}
	return str(ev.Stdout)
}

type importLine struct {
	Event *Event `json:"event"`
	Src   Source `json:"src"`
}

// Import reads NDJSON lines of {"event":…, "src":…} and stores them.
func (s *Store) Import(r io.Reader, progress func(int)) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	btx, err := s.blobs.Begin()
	if err != nil {
		return 0, err
	}
	n := 0
	commit := func() error {
		if err := btx.Commit(); err != nil {
			return err
		}
		return tx.Commit()
	}
	for sc.Scan() {
		var line importLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			return n, fmt.Errorf("line %d: %w", n+1, err)
		}
		if line.Event == nil || line.Event.Type != "command_execution" {
			return n, fmt.Errorf("line %d: unsupported event", n+1)
		}
		data, hash, err := Canonical(line.Event)
		if err != nil {
			return n, err
		}
		if err := s.putBlob(btx, hash, data); err != nil {
			return n, err
		}
		ev := line.Event
		out := ev.MainText()
		var id int64
		err = tx.QueryRow(`SELECT id FROM events WHERE hash = ?`, hash).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			cmd := ev.Command
			if len(cmd) > 400 {
				cmd = cmd[:400]
			}
			res, err := tx.Exec(`INSERT INTO events(hash, program, cmd, exit_code, out_len, out_lines, err_len, truncated, error) VALUES(?,?,?,?,?,?,?,?,?)`,
				hash, Program(ev.Command), cmd, ev.ExitCode, len(out), lineCount(out), len(str(ev.Stderr)), ev.Truncated, ev.Error != nil)
			if err != nil {
				return n, err
			}
			id, _ = res.LastInsertId()
			ftsOut := out
			if len(ftsOut) > 8192 {
				ftsOut = ftsOut[:8192]
			}
			if _, err := tx.Exec(`INSERT INTO events_fts(rowid, cmd, out) VALUES(?,?,?)`, id, ev.Command, ftsOut+"\n"+str(ev.Stderr)); err != nil {
				return n, err
			}
		} else if err != nil {
			return n, err
		}
		meta := string(line.Src.Meta)
		if meta == "" {
			meta = "{}"
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sources(event, source, unit, split, turn, call, meta) VALUES(?,?,?,?,?,?,?)`,
			id, line.Src.Source, line.Src.Unit, splitOf(line.Src), line.Src.Turn, line.Src.Call, meta); err != nil {
			return n, err
		}
		n++
		if n%5000 == 0 {
			if err := commit(); err != nil {
				return n, err
			}
			if tx, err = s.db.Begin(); err != nil {
				return n, err
			}
			if btx, err = s.blobs.Begin(); err != nil {
				return n, err
			}
		}
		if progress != nil {
			progress(n)
		}
	}
	if err := sc.Err(); err != nil {
		return n, err
	}
	return n, commit()
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// Analyze recomputes base rates (05/B1) and per-event exit-code surprisal.
func (s *Store) Analyze() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`DELETE FROM stats`,
		`INSERT INTO stats SELECT 'exit_code', COALESCE(CAST(exit_code AS TEXT), 'none'), COUNT(*) FROM events GROUP BY 1, 2`,
		`INSERT INTO stats SELECT 'program', program, COUNT(*) FROM events GROUP BY 1, 2`,
		`INSERT INTO stats SELECT 'out_len_log2', CASE WHEN out_len = 0 THEN '0' ELSE CAST(CAST(log2(out_len) AS INTEGER) AS TEXT) END, COUNT(*) FROM events GROUP BY 1, 2`,
		`INSERT INTO stats SELECT 'truncated', CAST(truncated AS TEXT), COUNT(*) FROM events GROUP BY 1, 2`,
		`INSERT INTO stats SELECT 'error', CAST(error AS TEXT), COUNT(*) FROM events GROUP BY 1, 2`,
		`INSERT INTO stats SELECT 'has_stderr', CASE WHEN err_len > 0 THEN '1' ELSE '0' END, COUNT(*) FROM events GROUP BY 1, 2`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	var total float64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&total); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT value, count FROM stats WHERE field = 'exit_code'`)
	if err != nil {
		return err
	}
	type kv struct {
		v string
		s float64
	}
	var all []kv
	for rows.Next() {
		var v string
		var c float64
		rows.Scan(&v, &c)
		all = append(all, kv{v, -math.Log2(c / total)})
	}
	rows.Close()
	for _, x := range all {
		q := `UPDATE events SET surprise = ? WHERE exit_code = ?`
		var arg any = x.v
		if x.v == "none" {
			q = `UPDATE events SET surprise = ? WHERE exit_code IS NULL AND ? = 'none'`
		}
		if _, err := tx.Exec(q, x.s, arg); err != nil {
			return err
		}
	}
	return tx.Commit()
}
