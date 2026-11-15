package lexicon

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one line of a lexicon base file (09/Z6).
type Entry struct {
	Lemma   string            `json:"lemma"`
	POS     string            `json:"pos"` // noun, verb, adj
	Forms   map[string]string `json:"forms,omitempty"`
	More    bool              `json:"more,omitempty"`    // adjective compares with more/most
	Uncount bool              `json:"uncount,omitempty"` // noun has no ordinary plural
	An      bool              `json:"an,omitempty"`      // takes "an" (starts with a vowel sound)
	Doubt   []string          `json:"doubt,omitempty"`   // which forms the rules guessed (05/P1d.2)
	Src     []string          `json:"src"`
}

// Form slots by part of speech.
var Slots = map[string][]string{
	"noun": {"plural"},
	"verb": {"pres3", "past", "pastp", "ing"},
	"adj":  {"er", "est"},
}

// FormRef says which entry and slot a surface word is ("gave" is give, past).
type FormRef struct {
	Lemma, POS, Slot string // Slot "" means the lemma itself
}

// Lexicon is a loaded base plus a reverse index from surface forms.
type Lexicon struct {
	Hash    string
	entries map[string]*Entry // pos|lemma
	forms   map[string][]FormRef
}

func key(pos, lemma string) string { return pos + "|" + lemma }

// Get returns the entry for a lemma and part of speech.
func (l *Lexicon) Get(pos, lemma string) *Entry { return l.entries[key(pos, lemma)] }

// Lookup returns every entry and slot a surface word could be.
func (l *Lexicon) Lookup(word string) []FormRef { return l.forms[strings.ToLower(word)] }

// Len is the number of entries.
func (l *Lexicon) Len() int { return len(l.entries) }

// Put adds or replaces an entry, e.g. an override from the grammar store (09/Z4b).
func (l *Lexicon) Put(e *Entry) {
	if old := l.entries[key(e.POS, e.Lemma)]; old != nil {
		l.unindex(old)
	}
	l.entries[key(e.POS, e.Lemma)] = e
	l.index(e)
}

func (l *Lexicon) index(e *Entry) {
	l.forms[e.Lemma] = append(l.forms[e.Lemma], FormRef{e.Lemma, e.POS, ""})
	for _, slot := range Slots[e.POS] {
		if f := e.Forms[slot]; f != "" {
			l.forms[f] = append(l.forms[f], FormRef{e.Lemma, e.POS, slot})
		}
	}
}

func (l *Lexicon) unindex(e *Entry) {
	drop := func(w string) {
		refs := l.forms[w][:0]
		for _, r := range l.forms[w] {
			if r.Lemma != e.Lemma || r.POS != e.POS {
				refs = append(refs, r)
			}
		}
		l.forms[w] = refs
	}
	drop(e.Lemma)
	for _, f := range e.Forms {
		drop(f)
	}
}

// New returns an empty lexicon (for tests and overrides only).
func New() *Lexicon {
	return &Lexicon{entries: map[string]*Entry{}, forms: map[string][]FormRef{}}
}

// Load reads a base file.
func Load(path string) (*Lexicon, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l := New()
	h := sha256.New()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		h.Write(sc.Bytes())
		h.Write([]byte{'\n'})
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		l.Put(&e)
	}
	l.Hash = hex.EncodeToString(h.Sum(nil))
	return l, sc.Err()
}

// Latest finds the newest base file in dir (by modification time).
func Latest(dir string) (string, error) {
	paths, _ := filepath.Glob(filepath.Join(dir, "base-*.jsonl"))
	if len(paths) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(paths, func(i, j int) bool {
		a, _ := os.Stat(paths[i])
		b, _ := os.Stat(paths[j])
		return a.ModTime().After(b.ModTime())
	})
	return paths[0], nil
}

// Write stores entries as a base file named by its hash and returns the path.
func Write(dir string, entries []*Entry) (string, string, error) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].POS != entries[j].POS {
			return entries[i].POS < entries[j].POS
		}
		return entries[i].Lemma < entries[j].Lemma
	})
	var buf strings.Builder
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			return "", "", err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(buf.String()))
	hash := hex.EncodeToString(sum[:])
	p := filepath.Join(dir, "base-"+hash+".jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(buf.String()), 0o644); err != nil {
		return "", "", err
	}
	return p, hash, os.Rename(tmp, p)
}
