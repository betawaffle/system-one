package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// SampleSet is something to draw random samples from: an utterance source, a
// command source, or a derived dataset file (datasets/<name>/<split>.jsonl).
type SampleSet struct {
	ID    string // "u:<source>", "c:<source>", "d:<name>/<split>"
	Group string
	Label string
	Count int
}

func (s *Store) SampleSets() ([]SampleSet, error) {
	var out []SampleSet
	rows, err := s.db.Query(`SELECT source, COUNT(DISTINCT utterance) FROM usources GROUP BY source ORDER BY source`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var src string
		var n int
		rows.Scan(&src, &n)
		out = append(out, SampleSet{ID: "u:" + src, Group: "Utterances", Label: src, Count: n})
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT source, COUNT(DISTINCT event) FROM sources GROUP BY source ORDER BY source`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var src string
		var n int
		rows.Scan(&src, &n)
		out = append(out, SampleSet{ID: "c:" + src, Group: "Commands", Label: src, Count: n})
	}
	rows.Close()
	files, _ := filepath.Glob(filepath.Join(s.dir, "datasets", "*", "*.jsonl"))
	sort.Strings(files)
	for _, f := range files {
		rel, _ := filepath.Rel(filepath.Join(s.dir, "datasets"), f)
		name := strings.TrimSuffix(rel, ".jsonl")
		idx, err := s.lineIndex(f)
		if err != nil {
			continue
		}
		out = append(out, SampleSet{ID: "d:" + name, Group: "Derived Datasets", Label: name, Count: len(idx)})
	}
	return out, nil
}

// Sample is one drawn item. Text is cut at MaxSampleText (the full item is a link away).
type Sample struct {
	Kind      string // utterance, command, line
	Hash      string // utterance or event hash, when there is one
	Speaker   string
	What      string // kind for utterances, program for commands
	Model     string
	Source    string
	Split     string
	Text      string
	Cut       int // characters not shown
	ExitCode  *int
	Meta      map[string]any
	LineIndex int
}

const MaxSampleText = 6000

func cut(t string) (string, int) {
	if len(t) <= MaxSampleText {
		return t, 0
	}
	c := truncUTF8(t, MaxSampleText)
	return c, len(t) - len(c)
}

// picks returns n distinct indexes in [0, total), deterministic for a seed.
func picks(total, n int, seed uint64) []int {
	r := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	if n >= total {
		out := r.Perm(total)
		return out
	}
	seen := map[int]bool{}
	var out []int
	for len(out) < n {
		i := r.IntN(total)
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}

// Draw returns n random samples from a set, plus how many items the set (with
// filters) holds. Same set, filters and seed give the same draw.
func (s *Store) Draw(set, speaker, kind string, n int, seed uint64) ([]Sample, int, error) {
	switch {
	case strings.HasPrefix(set, "u:"):
		return s.drawUtterances(set[2:], speaker, kind, n, seed)
	case strings.HasPrefix(set, "c:"):
		return s.drawCommands(set[2:], n, seed)
	case strings.HasPrefix(set, "d:"):
		return s.drawLines(set[2:], speaker, kind, n, seed)
	}
	return nil, 0, fmt.Errorf("unknown set %q", set)
}

func (s *Store) drawUtterances(source, speaker, kind string, n int, seed uint64) ([]Sample, int, error) {
	where := `s.source = ?`
	args := []any{source}
	if speaker != "" {
		where += ` AND u.speaker = ?`
		args = append(args, speaker)
	}
	if kind != "" {
		where += ` AND u.kind = ?`
		args = append(args, kind)
	}
	from := ` FROM usources s JOIN utterances u ON u.id = s.utterance WHERE ` + where
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	var out []Sample
	for _, i := range picks(total, n, seed) {
		var sm Sample
		var meta string
		err := s.db.QueryRow(`SELECT u.hash, u.speaker, u.kind, COALESCE(u.model,''), s.source, s.split, s.meta`+from+` ORDER BY s.rowid LIMIT 1 OFFSET ?`,
			append(args, i)...).Scan(&sm.Hash, &sm.Speaker, &sm.What, &sm.Model, &sm.Source, &sm.Split, &meta)
		if err != nil {
			return nil, 0, err
		}
		b, err := s.Blob(sm.Hash)
		if err != nil {
			return nil, 0, err
		}
		var u Utterance
		json.Unmarshal(b, &u)
		sm.Kind = "utterance"
		sm.Text, sm.Cut = cut(u.Text)
		json.Unmarshal([]byte(meta), &sm.Meta)
		out = append(out, sm)
	}
	return out, total, nil
}

func (s *Store) drawCommands(source string, n int, seed uint64) ([]Sample, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sources WHERE source = ?`, source).Scan(&total); err != nil {
		return nil, 0, err
	}
	var out []Sample
	for _, i := range picks(total, n, seed) {
		var sm Sample
		var meta string
		err := s.db.QueryRow(`SELECT e.hash, e.program, s.split, s.meta FROM sources s JOIN events e ON e.id = s.event WHERE s.source = ? ORDER BY s.rowid LIMIT 1 OFFSET ?`,
			source, i).Scan(&sm.Hash, &sm.What, &sm.Split, &meta)
		if err != nil {
			return nil, 0, err
		}
		ev, err := s.Event(sm.Hash)
		if err != nil {
			return nil, 0, err
		}
		sm.Kind, sm.Source, sm.ExitCode = "command", source, ev.ExitCode
		sm.Text, sm.Cut = cut(ev.Command + "\n\n" + ev.MainText())
		json.Unmarshal([]byte(meta), &sm.Meta)
		out = append(out, sm)
	}
	return out, total, nil
}

var lineIdx sync.Map // path -> lineIndexEntry

type lineIndexEntry struct {
	size, mtime int64
	offsets     []int64
}

// lineIndex returns the start offset of every line in a JSONL file, cached until it changes.
func (s *Store) lineIndex(path string) ([]int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if v, ok := lineIdx.Load(path); ok {
		e := v.(lineIndexEntry)
		if e.size == st.Size() && e.mtime == st.ModTime().UnixNano() {
			return e.offsets, nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var offs []int64
	var pos int64
	for {
		line, err := r.ReadSlice('\n')
		if len(line) > 0 {
			offs = append(offs, pos)
			pos += int64(len(line))
		}
		if err == bufio.ErrBufferFull {
			// long line: keep reading it without recording a new start
			for err == bufio.ErrBufferFull {
				line, err = r.ReadSlice('\n')
				pos += int64(len(line))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	lineIdx.Store(path, lineIndexEntry{st.Size(), st.ModTime().UnixNano(), offs})
	return offs, nil
}

func (s *Store) drawLines(name, speaker, kind string, n int, seed uint64) ([]Sample, int, error) {
	path := filepath.Join(s.dir, "datasets", filepath.Clean("/"+name)+".jsonl")
	offs, err := s.lineIndex(path)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	read := func(i int) (map[string]any, error) {
		f.Seek(offs[i], io.SeekStart)
		line, err := bufio.NewReader(f).ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		m := map[string]any{}
		return m, json.Unmarshal(line, &m)
	}
	str := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
	var out []Sample
	total := len(offs)
	filtered := speaker != "" || kind != ""
	tries := 0
	for _, i := range picks(total, min(total, n*50), seed) {
		if len(out) >= n {
			break
		}
		tries++
		m, err := read(i)
		if err != nil {
			continue
		}
		if (speaker != "" && str(m, "speaker") != speaker) || (kind != "" && str(m, "kind") != kind) {
			continue
		}
		sm := Sample{Kind: "line", Hash: str(m, "utterance"), Speaker: str(m, "speaker"), What: str(m, "kind"), Model: str(m, "model"),
			Source: str(m, "source"), LineIndex: i, Meta: m}
		sm.Text, sm.Cut = cut(str(m, "text"))
		out = append(out, sm)
	}
	if filtered {
		total = -1 // unknown without a full scan
	}
	return out, total, nil
}
