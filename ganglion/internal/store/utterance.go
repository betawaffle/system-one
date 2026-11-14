package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Utterance is one piece of text with who produced it (11/U1). Who-said-what is
// decided by the importers from structure (tags, roles, flags), never by guessing.
type Utterance struct {
	Type    string  `json:"type"`    // "utterance"
	Speaker string  `json:"speaker"` // human (the person in our sessions), claude, subagent, model (other models), author (third-party people), mixed, harness
	Kind    string  `json:"kind"`    // typed, pasted, injected, summary, reply, thinking, prompt, document, commit
	Model   *string `json:"model"`   // the model's own ID when the speaker is a model, if known
	Tag     *string `json:"tag"`     // for injected text: the tag or mechanism that marked it
	Text    string  `json:"text"`
}

const utteranceSchema = `
CREATE TABLE IF NOT EXISTS utterances (
	id INTEGER PRIMARY KEY,
	hash TEXT NOT NULL UNIQUE,
	speaker TEXT NOT NULL,
	kind TEXT NOT NULL,
	model TEXT,
	tag TEXT,
	chars INTEGER NOT NULL,
	words INTEGER NOT NULL,
	preview TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS utterances_who ON utterances(speaker, kind);
CREATE TABLE IF NOT EXISTS usources (
	utterance INTEGER NOT NULL REFERENCES utterances(id),
	source TEXT NOT NULL,
	unit TEXT NOT NULL,
	split TEXT NOT NULL,
	turn INTEGER NOT NULL,
	call INTEGER NOT NULL,
	at TEXT,
	meta TEXT NOT NULL,
	UNIQUE(source, unit, turn, call)
);
CREATE INDEX IF NOT EXISTS usources_utterance ON usources(utterance);
CREATE INDEX IF NOT EXISTS usources_source ON usources(source, split);
CREATE VIRTUAL TABLE IF NOT EXISTS utterances_fts USING fts5(text, content='', tokenize='unicode61');
`

func canonicalUtterance(u *Utterance) ([]byte, string, error) {
	b, err := json.Marshal(u)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return b, fmt.Sprintf("%x", sum[:]), nil
}

// utteranceSplit holds out 1 in 10 units (sessions, conversations, documents),
// for every source: our own text is training data here, unlike local commands.
func utteranceSplit(src Source) string {
	key := src.SplitKey
	if key == "" {
		key = src.Unit
	}
	sum := sha256.Sum256([]byte(src.Source + "\x00" + key))
	if sum[0]%10 == 0 {
		return "test"
	}
	return "train"
}

func (s *Store) importUtterance(tx, btx *sql.Tx, line importLine) error {
	var u Utterance
	if err := json.Unmarshal(line.Event, &u); err != nil {
		return err
	}
	if strings.TrimSpace(u.Text) == "" {
		return nil
	}
	data, hash, err := canonicalUtterance(&u)
	if err != nil {
		return err
	}
	if err := s.putBlob(btx, hash, data); err != nil {
		return err
	}
	var id int64
	err = tx.QueryRow(`SELECT id FROM utterances WHERE hash = ?`, hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		preview := strings.Join(strings.Fields(truncUTF8(u.Text, 600)), " ")
		res, err := tx.Exec(`INSERT INTO utterances(hash, speaker, kind, model, tag, chars, words, preview) VALUES(?,?,?,?,?,?,?,?)`,
			hash, u.Speaker, u.Kind, u.Model, u.Tag, len(u.Text), len(strings.Fields(u.Text)), truncUTF8(preview, 300))
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if _, err := tx.Exec(`INSERT INTO utterances_fts(rowid, text) VALUES(?,?)`, id, truncUTF8(u.Text, 65536)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var meta struct {
		At string `json:"at"`
	}
	json.Unmarshal(line.Src.Meta, &meta)
	_, err = tx.Exec(`INSERT OR IGNORE INTO usources(utterance, source, unit, split, turn, call, at, meta) VALUES(?,?,?,?,?,?,?,?)`,
		id, line.Src.Source, line.Src.Unit, utteranceSplit(line.Src), line.Src.Turn, line.Src.Call, nullIfEmpty(meta.At), metaOf(line.Src))
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// URow is one utterance in a list.
type URow struct {
	ID        int64
	Hash      string
	Speaker   string
	Kind      string
	Model     string
	Tag       string
	Chars     int
	Words     int
	Preview   string
	Sightings int
	At        string
}

// UFilter narrows an utterance list.
type UFilter struct {
	Q, Speaker, Kind, Source, Split string
	Limit                           int
}

func (f UFilter) where() (string, []any) {
	var w []string
	var args []any
	if f.Q != "" {
		w = append(w, `u.id IN (SELECT rowid FROM utterances_fts WHERE utterances_fts MATCH ?)`)
		args = append(args, ftsQuery(f.Q))
	}
	if f.Speaker != "" {
		w = append(w, `u.speaker = ?`)
		args = append(args, f.Speaker)
	}
	if f.Kind != "" {
		w = append(w, `u.kind = ?`)
		args = append(args, f.Kind)
	}
	if f.Source != "" || f.Split != "" {
		q := `u.id IN (SELECT utterance FROM usources WHERE 1=1`
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
	if len(w) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w, " AND "), args
}

// Utterances lists utterances, newest sighting first.
func (s *Store) Utterances(f UFilter, offset int) ([]URow, bool, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where, args := f.where()
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM utterances u`+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	q := `SELECT u.id, u.hash, u.speaker, u.kind, COALESCE(u.model,''), COALESCE(u.tag,''), u.chars, u.words, u.preview,
		(SELECT COUNT(*) FROM usources s WHERE s.utterance = u.id), COALESCE((SELECT MAX(at) FROM usources s WHERE s.utterance = u.id), '')
		FROM utterances u` + where + fmt.Sprintf(` ORDER BY 11 DESC, u.id DESC LIMIT %d OFFSET %d`, f.Limit+1, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var out []URow
	for rows.Next() {
		var r URow
		if err := rows.Scan(&r.ID, &r.Hash, &r.Speaker, &r.Kind, &r.Model, &r.Tag, &r.Chars, &r.Words, &r.Preview, &r.Sightings, &r.At); err != nil {
			return nil, false, 0, err
		}
		out = append(out, r)
	}
	more := len(out) > f.Limit
	if more {
		out = out[:f.Limit]
	}
	return out, more, total, rows.Err()
}

// LookupUtterance finds one by hash prefix (at least 8 characters).
func (s *Store) LookupUtterance(hash string) (*URow, *Utterance, []Source, error) {
	if len(hash) < 8 {
		return nil, nil, nil, sql.ErrNoRows
	}
	var r URow
	err := s.db.QueryRow(`SELECT id, hash, speaker, kind, COALESCE(model,''), COALESCE(tag,''), chars, words, preview FROM utterances WHERE hash >= ? AND hash < ? LIMIT 1`,
		hash, hash+"\xff").Scan(&r.ID, &r.Hash, &r.Speaker, &r.Kind, &r.Model, &r.Tag, &r.Chars, &r.Words, &r.Preview)
	if err != nil {
		return nil, nil, nil, err
	}
	b, err := s.Blob(r.Hash)
	if err != nil {
		return nil, nil, nil, err
	}
	var u Utterance
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, nil, nil, err
	}
	rows, err := s.db.Query(`SELECT source, unit, split, turn, call, meta FROM usources WHERE utterance = ? ORDER BY at, source LIMIT 100`, r.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	var srcs []Source
	for rows.Next() {
		var x Source
		var meta string
		rows.Scan(&x.Source, &x.Unit, &x.Split, &x.Turn, &x.Call, &meta)
		x.Meta = []byte(meta)
		srcs = append(srcs, x)
	}
	r.Sightings = len(srcs)
	return &r, &u, srcs, rows.Err()
}

// WhoCount is one row of the speaker/kind breakdown.
type WhoCount struct {
	Speaker, Kind string
	N, Chars      int
}

func (s *Store) WhoCounts() ([]WhoCount, error) {
	rows, err := s.db.Query(`SELECT speaker, kind, COUNT(*), SUM(chars) FROM utterances GROUP BY speaker, kind ORDER BY speaker, SUM(chars) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WhoCount
	for rows.Next() {
		var c WhoCount
		rows.Scan(&c.Speaker, &c.Kind, &c.N, &c.Chars)
		out = append(out, c)
	}
	return out, rows.Err()
}

// USourceNames lists utterance sources.
func (s *Store) USourceNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT source FROM usources ORDER BY source`)
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
