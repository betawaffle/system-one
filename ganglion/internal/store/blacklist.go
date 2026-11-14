package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Blacklist entries exclude an utterance of Claude's own output from every
// derived dataset (11/B1). The file is append-only JSONL in the data directory;
// an entry names the utterance by hash and says why.
type BlacklistEntry struct {
	Hash   string `json:"hash"`
	Reason string `json:"reason"`
	Added  string `json:"added"`
	By     string `json:"by"`
}

func (s *Store) blacklistPath() string { return filepath.Join(s.dir, "blacklist.jsonl") }

// Blacklist returns entries by utterance hash. The file is append-only, so a
// later entry for the same hash replaces an earlier one (corrections).
func (s *Store) Blacklist() (map[string]BlacklistEntry, error) {
	out := map[string]BlacklistEntry{}
	f, err := os.Open(s.blacklistPath())
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		var e BlacklistEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Hash != "" {
			out[e.Hash] = e
		}
	}
	return out, sc.Err()
}

// AddToBlacklist blacklists one of Claude's own utterances, found by hash prefix.
// Other speakers' text can't be blacklisted here: this list is for Claude's output.
func (s *Store) AddToBlacklist(prefix, reason, by string) (*BlacklistEntry, *URow, error) {
	row, _, _, err := s.LookupUtterance(prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("no utterance with hash prefix %q", prefix)
	}
	if row.Speaker != "claude" && row.Speaker != "subagent" {
		return nil, row, fmt.Errorf("utterance %s is %s's, not Claude's own output", row.Hash[:10], row.Speaker)
	}
	if reason == "" {
		return nil, row, fmt.Errorf("a reason is required")
	}
	e := BlacklistEntry{Hash: row.Hash, Reason: reason, Added: time.Now().UTC().Format(time.RFC3339), By: by}
	b, _ := json.Marshal(e)
	f, err := os.OpenFile(s.blacklistPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, row, err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return nil, row, err
	}
	return &e, row, f.Sync()
}
