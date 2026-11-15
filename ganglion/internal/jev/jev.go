// Package jev asks TypeSafe's Jev the narrow checks as one request: the sample
// card as the state, each check as a typed question, plus a direct verdict for
// comparison (design/synthetic-data/10/J14). Code makes the verdict from the
// checks, the same way as the local model's (10/J3).
package jev

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ganglion/internal/judge"
	"ganglion/internal/llm"
)

//go:embed questions.json
var questionsFile []byte

// State is what Jev sees for one sample: the sample and the structure of the
// meaning it was generated from, as JSON, so questions can name fields.
type State struct {
	Sample  string  `json:"sample"`
	Meaning Meaning `json:"meaning"`
	// Implications the rules already tag for this sample (02/L9).
	AlreadyRecorded []string `json:"already_recorded_implications"`
	Author          Author   `json:"author_example"`
}

type Meaning struct {
	Frame          string          `json:"frame"`
	FrameExplainer string          `json:"frame_explainer"`
	FrameKind      string          `json:"frame_kind"` // state: holds over time; action: an event
	Roles          map[string]Role `json:"roles"`
	Tense          string          `json:"tense"`
	Polarity       string          `json:"polarity"` // positive affirms the frame; negative denies it
	Mood           string          `json:"mood"`     // statement or question
	Aspect         string          `json:"aspect,omitempty"`
	Time           string          `json:"time,omitempty"` // a time modifier the sample adds
}

type Role struct {
	Explainer string `json:"explainer"`
	FilledBy  string `json:"filled_by,omitempty"` // as the sample refers to it
	Type      string `json:"type,omitempty"`      // the entity's type
	Attribute string `json:"attribute,omitempty"`
	Value     string `json:"value,omitempty"`
	Direction string `json:"direction,omitempty"` // compare: more, less, same
}

type Author struct {
	Sentence string `json:"sentence"`
	About    string `json:"about"`
}

// AuthorAbout explains the author example's field.
const AuthorAbout = "A sentence the author wrote for this frame, known to be good. The sample varies its tense, polarity, mood and fillers on purpose."

const combineVersion = "jev-checks-2"

// Version is the hash of the questions, the state text and the combining rules.
var Version = func() string {
	h := sha256.New()
	st, _ := json.Marshal(State{})
	h.Write([]byte(combineVersion + "\x00" + AuthorAbout + "\x00"))
	h.Write(st) // the state's shape
	h.Write([]byte{0})
	h.Write(questionsFile)
	return "jev-" + hex.EncodeToString(h.Sum(nil))[:8]
}()

// Model is what we ask for; the response names the exact version.
const Model = "jev-latest"

// Config is the key file: line 1 the API key, line 2 the endpoint.
type Config struct{ Key, URL, Dir string }

func Load(path, dir string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" || !strings.HasPrefix(strings.TrimSpace(lines[1]), "https://") {
		return nil, fmt.Errorf("%s: want the key on line 1 and the https endpoint on line 2", path)
	}
	return &Config{Key: strings.TrimSpace(lines[0]), URL: strings.TrimSpace(lines[1]), Dir: dir}, nil
}

// Request is the exact body sent for one sample.
func Request(st State) ([]byte, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, questionsFile); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		State     State           `json:"state"`
		Model     string          `json:"model"`
		Questions json.RawMessage `json:"questions"`
	}{st, Model, compact.Bytes()})
}

type answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// flags: the answer to each check that counts against the sample, and what it gives.
var flags = []struct {
	q, bad          string
	verdict, reason string
	butKind         string
}{
	{q: "grammar", bad: "broken", verdict: "bad", reason: "Ungrammatical"},
	{q: "filler", bad: "wrong_kind", verdict: "bad", reason: "Wrong Kind of Filler"},
	{q: "meaning", bad: "differs", verdict: "bad", reason: "Means Something Else"},
	{q: "change", bad: "impossible_change", verdict: "bad", reason: "Other"},
	{q: "implies", bad: "implies_more", verdict: "but", butKind: "implies"},
	{q: "context", bad: "needs_earlier", verdict: "but", butKind: "context"},
}

var direct = map[string]judge.Verdict{
	"fine": {Verdict: "ok"}, "fine_needs_context": {Verdict: "but", ButKind: "context"}, "fine_implies_more": {Verdict: "but", ButKind: "implies"},
	"word_form": {Verdict: "bad", Reason: "Word Form"}, "wrong_kind_of_filler": {Verdict: "bad", Reason: "Wrong Kind of Filler"},
	"ungrammatical": {Verdict: "bad", Reason: "Ungrammatical"}, "means_something_else": {Verdict: "bad", Reason: "Means Something Else"},
	"sounds_odd": {Verdict: "bad", Reason: "Sounds Odd"}, "other": {Verdict: "bad", Reason: "Other"},
}

func level(c float64) string {
	switch {
	case c >= 0.8:
		return "high"
	case c >= 0.5:
		return "medium"
	}
	return "low"
}

// Result is the combined verdict, each check, and Jev's direct verdict.
type Result struct {
	Verdict judge.Verdict
	Checks  []judge.Check
	Direct  judge.Verdict
	Call    string
}

// Judge sends one sample and combines the answers.
func (c *Config) Judge(ctx context.Context, st State) (Result, error) {
	raw, err := Request(st)
	if err != nil {
		return Result{}, err
	}
	rec, err := llm.Post(ctx, c.Dir, "typesafe", "judge", Version, c.URL, c.Key, Model, raw)
	if err != nil {
		return Result{Call: rec.ID}, err
	}
	var resp struct {
		Answers map[string]answer `json:"answers"`
	}
	if err := json.Unmarshal([]byte(rec.Response), &resp); err != nil {
		return Result{Call: rec.ID}, fmt.Errorf("the answer didn't fit: %w", err)
	}
	return combine(resp.Answers, rec.ID)
}

func combine(as map[string]answer, call string) (Result, error) {
	r := Result{Call: call}
	var decided *judge.Verdict
	lowest := 1.0
	for _, f := range flags {
		a, ok := as[f.q]
		if !ok || a.Choice == "" {
			return r, fmt.Errorf("the answer didn't fit: no answer to %s", f.q)
		}
		ch := judge.Check{Name: f.q, Answer: a.Choice, Confidence: fmt.Sprintf("%.2f", a.Confidence), Flagged: a.Choice == f.bad, Call: call,
			Why: fmt.Sprintf("p(%s) = %.2f", f.bad, a.Probabilities[f.bad])}
		r.Checks = append(r.Checks, ch)
		lowest = min(lowest, a.Confidence)
		if ch.Flagged && decided == nil {
			decided = &judge.Verdict{Verdict: f.verdict, Reason: f.reason, ButKind: f.butKind, Note: f.q + ": " + a.Choice, Confidence: level(a.Confidence), Decided: f.q}
		}
	}
	if decided != nil {
		r.Verdict = *decided
	} else {
		r.Verdict = judge.Verdict{Verdict: "ok", Confidence: level(lowest)}
	}
	if a, ok := as["verdict"]; ok {
		r.Direct = direct[a.Choice]
		r.Direct.Confidence = level(a.Confidence)
		r.Direct.Note = a.Choice
	}
	return r, nil
}
