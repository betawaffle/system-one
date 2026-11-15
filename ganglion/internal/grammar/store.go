// Package grammar stores the synthetic-data grammar: types, attributes,
// frames, entities, lexicon overrides and templates, as content-addressed
// versions (design/synthetic-data/07, shapes in 09).
//
// Files are the source of truth (07/D91): content blobs under objects/, and
// append-only versions.jsonl and events.jsonl. Everything else is rebuilt in
// memory at Open, so nothing here depends on a backup getting SQLite right.
package grammar

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Version is one line of versions.jsonl (09/Z2).
type Version struct {
	Schema int    `json:"schema"`
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Hash   string `json:"hash"`
	Parent string `json:"parent"`
	By     string `json:"by"`
	At     string `json:"at"`
	Why    string `json:"why"`
	Client string `json:"client,omitempty"`
	Event  string `json:"event,omitempty"`
}

// Meta is who and why for a write.
type Meta struct {
	By, Why, Client, Event string
}

var ErrNotFound = errors.New("not found")

type Store struct {
	dir     string
	mu      sync.Mutex
	heads   map[string]Version   // id -> current version; retired ids are absent
	history map[string][]Version // id -> all versions, oldest first
}

func Open(dir string) (*Store, error) {
	for _, d := range []string{"objects", "lexicon"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{dir: dir, heads: map[string]Version{}, history: map[string][]Version{}}
	for _, name := range []string{"versions.jsonl", "events.jsonl"} {
		if err := endTornLine(filepath.Join(dir, name)); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(filepath.Join(dir, "versions.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for n := 1; sc.Scan(); n++ {
		var v Version
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			// a torn last line from a crash mid-append is skipped, not fatal
			continue
		}
		s.apply(v)
	}
	return s, sc.Err()
}

// endTornLine appends a newline if a crash left the file mid-line, so the next
// record starts on its own line. Nothing is removed (append-only).
func endTornLine(p string) error {
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return err
	}
	last := make([]byte, 1)
	if _, err := f.ReadAt(last, st.Size()-1); err != nil {
		return err
	}
	if last[0] == '\n' {
		return nil
	}
	if _, err := f.WriteAt([]byte{'\n'}, st.Size()); err != nil {
		return err
	}
	return f.Sync()
}

func (s *Store) apply(v Version) {
	s.history[v.ID] = append(s.history[v.ID], v)
	if v.Why == "retire" {
		delete(s.heads, v.ID)
	} else {
		s.heads[v.ID] = v
	}
}

// Canonical is the hashed form of content (09/Z3).
func Canonical(content any) ([]byte, string, error) {
	b, err := json.Marshal(content)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

func (s *Store) objectPath(hash string) string {
	return filepath.Join(s.dir, "objects", hash[:2], hash+".json")
}

// PutObject stores content by hash and returns the hash. Existing objects are left alone.
func (s *Store) PutObject(content any) (string, error) {
	b, hash, err := Canonical(content)
	if err != nil {
		return "", err
	}
	return hash, s.writeObject(hash, b)
}

func (s *Store) writeObject(hash string, b []byte) error {
	p := s.objectPath(hash)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, p)
}

// Object returns a content blob's bytes.
func (s *Store) Object(hash string) ([]byte, error) {
	if len(hash) < 3 {
		return nil, ErrNotFound
	}
	b, err := os.ReadFile(s.objectPath(hash))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func (s *Store) appendLine(name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// Put writes a new version of id when its content differs from the head.
// Writing the same content again is a no-op and returns the head.
func (s *Store) Put(id, kind string, content any, m Meta) (Version, error) {
	if !strings.HasPrefix(id, kind+":") {
		return Version{}, fmt.Errorf("id %q must start with %q", id, kind+":")
	}
	b, hash, err := Canonical(content)
	if err != nil {
		return Version{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	head, ok := s.heads[id]
	if ok && head.Hash == hash {
		return head, nil
	}
	if err := s.writeObject(hash, b); err != nil {
		return Version{}, err
	}
	why := m.Why
	if why == "" {
		why = map[bool]string{true: "edit", false: "create"}[ok]
	}
	parent := head.Hash
	if !ok && len(s.history[id]) > 0 { // re-created after retiring
		parent = s.history[id][len(s.history[id])-1].Hash
	}
	v := Version{Schema: 1, ID: id, Kind: kind, Hash: hash, Parent: parent, By: m.By, At: now(), Why: why, Client: m.Client, Event: m.Event}
	if err := s.appendLine("versions.jsonl", v); err != nil {
		return Version{}, err
	}
	s.apply(v)
	return v, nil
}

// Retire ends id's life without deleting anything (07/V3).
func (s *Store) Retire(id string, m Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	head, ok := s.heads[id]
	if !ok {
		return ErrNotFound
	}
	v := Version{Schema: 1, ID: id, Kind: head.Kind, Hash: head.Hash, Parent: head.Hash, By: m.By, At: now(), Why: "retire", Client: m.Client, Event: m.Event}
	if err := s.appendLine("versions.jsonl", v); err != nil {
		return err
	}
	s.apply(v)
	return nil
}

// Head returns id's current version.
func (s *Store) Head(id string) (Version, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.heads[id]
	return v, ok
}

// Get decodes id's current content into out.
func (s *Store) Get(id string, out any) (Version, error) {
	v, ok := s.Head(id)
	if !ok {
		return v, ErrNotFound
	}
	b, err := s.Object(v.Hash)
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal(b, out)
}

// Heads lists current versions of one kind ("" for all), sorted by id.
func (s *Store) Heads(kind string) []Version {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Version
	for _, v := range s.heads {
		if kind == "" || v.Kind == kind {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// History lists every version of id, oldest first.
func (s *Store) History(id string) []Version {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Version(nil), s.history[id]...)
}

// SnapshotContent names one exact state of the grammar (07/N1).
type SnapshotContent struct {
	Base  string      `json:"base"`
	Heads [][2]string `json:"heads"`
}

// TakeSnapshot stores the current heads plus a lexicon base and returns its hash.
func (s *Store) TakeSnapshot(base string) (string, error) {
	var sc SnapshotContent
	sc.Base = base
	for _, v := range s.Heads("") {
		sc.Heads = append(sc.Heads, [2]string{v.ID, v.Hash})
	}
	return s.PutObject(sc)
}

// Event is one authoring step (07/E1). Data holds the event's own fields.
type Event struct {
	Schema int            `json:"schema"`
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	By     string         `json:"by"`
	At     string         `json:"at"`
	Client string         `json:"client"`
	Data   map[string]any `json:"data"`
}

// AddEvent appends an authoring event and returns its id.
func (s *Store) AddEvent(typ, by, client string, data map[string]any) (string, error) {
	var r [8]byte
	if _, err := rand.Read(r[:]); err != nil {
		return "", err
	}
	ev := Event{Schema: 1, ID: hex.EncodeToString(r[:]), Type: typ, By: by, At: now(), Client: client, Data: data}
	s.mu.Lock()
	defer s.mu.Unlock()
	return ev.ID, s.appendLine("events.jsonl", ev)
}

// Dir is the grammar directory.
func (s *Store) Dir() string { return s.dir }
