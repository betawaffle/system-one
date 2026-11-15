package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Labels are a person's judgements of samples (11/R1). They aren't derivable
// from anything else, so they live in an append-only JSONL file in the data
// directory, never in the rebuildable index. A later label for the same target
// and labeller replaces an earlier one; nothing is ever rewritten.
type Label struct {
	Target  string         `json:"target"`  // u:<utterance hash>, c:<event hash>, s:<sha256 of sentence text>
	Rating  string         `json:"rating"`  // good, bad, not-prose, or "" to clear
	By      string         `json:"by"`      // a name claim; no logins yet
	At      string         `json:"at"`      // UTC
	Set     string         `json:"set"`     // the dataset it was drawn from
	Seed    uint64         `json:"seed"`    // the draw it was seen in
	Context map[string]any `json:"context"` // what the page showed about it (source, speaker, kind, line, utterance)
	Client  string         `json:"client"`  // which UI
}

var Ratings = map[string]bool{"good": true, "bad": true, "not-prose": true, "": true}

var labelMu sync.Mutex

func (s *Store) labelsPath() string { return filepath.Join(s.dir, "labels.jsonl") }

// SentenceTarget names a sentence by its text, so labels survive dataset rebuilds.
func SentenceTarget(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "s:" + hex.EncodeToString(sum[:])
}

// AddLabel appends a label.
func (s *Store) AddLabel(l Label) error {
	if !Ratings[l.Rating] {
		return fmt.Errorf("unknown rating %q", l.Rating)
	}
	if len(l.Target) < 10 || (l.Target[:2] != "u:" && l.Target[:2] != "c:" && l.Target[:2] != "s:") {
		return fmt.Errorf("bad target %q", l.Target)
	}
	l.At = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	labelMu.Lock()
	defer labelMu.Unlock()
	f, err := os.OpenFile(s.labelsPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// CurrentLabels returns the latest rating per target for one labeller, and
// counts of current ratings.
func (s *Store) CurrentLabels(by string) (map[string]string, map[string]int, error) {
	cur := map[string]string{}
	f, err := os.Open(s.labelsPath())
	if os.IsNotExist(err) {
		return cur, map[string]int{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<24)
	for sc.Scan() {
		var l Label
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.By != by {
			continue
		}
		if l.Rating == "" {
			delete(cur, l.Target)
		} else {
			cur[l.Target] = l.Rating
		}
	}
	counts := map[string]int{}
	for _, r := range cur {
		counts[r]++
	}
	return cur, counts, sc.Err()
}
