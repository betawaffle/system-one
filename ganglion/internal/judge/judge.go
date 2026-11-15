// Package judge is the model's first pass over samples (design/synthetic-
// data/10/M1): narrow yes/no checks, one call each, schema-constrained, and
// code that turns their answers into a verdict (10/J1–J6). The prompt is a
// file; its hash plus the combining rules' version is the prompt version.
package judge

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"text/template"

	"ganglion/internal/llm"
)

//go:embed prompts/checks.md
var promptFile string

var tmpl = template.Must(template.New("judge").Parse(promptFile))

// PromptFile is the prompt template itself, for the page to show.
func PromptFile() string { return promptFile }

// combineVersion changes whenever Checks' order or mapping, or combine, changes.
const combineVersion = "checks-3"

// Version is the hash of the prompt file and the combining rules, recorded
// with every call and verdict.
var Version = func() string {
	s := sha256.Sum256([]byte(combineVersion + "\x00" + promptFile))
	return hex.EncodeToString(s[:])[:12]
}()

// Input is everything the prompt shows for one sample.
type Input struct {
	Frame, Explainer string
	Kind             string // the frame's: state | action
	Roles            []string
	Authored         string
	Sample           string
	Tags             string
	Known            string    // implications the rules already tag
	Notes            []string  // the person's earlier ◐ notes
	Examples         []Example // the person's verdicts on similar samples (off by default, 10/D113)
}

type Example struct{ Text, Verdict string }

// Verdict is what the model must return.
type Verdict struct {
	Verdict    string `json:"verdict"`
	ButKind    string `json:"but_kind"`
	Reason     string `json:"reason"`
	Note       string `json:"note"`
	Confidence string `json:"confidence"`
	Decided    string `json:"decided,omitempty"` // the check that gave it; empty for ✓
}

var Reasons = []string{"Word Form", "Wrong Kind of Filler", "Ungrammatical", "Means Something Else", "Sounds Odd", "Other"}

// A check is one narrow question; Flag on a yes gives the verdict.
type checkDef struct {
	Name, Title     string
	Verdict, Reason string // for ✕
	ButKind         string // for ◐
}

// Checks in the order they decide (10/J3): the first ✕ wins, then ◐.
var Checks = []checkDef{
	{Name: "form", Title: "Word Form", Verdict: "bad", Reason: "Word Form"},
	{Name: "grammar", Title: "Grammar", Verdict: "bad", Reason: "Ungrammatical"},
	{Name: "filler", Title: "Filler", Verdict: "bad", Reason: "Wrong Kind of Filler"},
	{Name: "meaning", Title: "Meaning", Verdict: "bad", Reason: "Means Something Else"},
	{Name: "change", Title: "Change", Verdict: "bad", Reason: "Other"},
	{Name: "implies", Title: "Implies", Verdict: "but", ButKind: "implies"},
	{Name: "context", Title: "Context", Verdict: "but", ButKind: "context"},
}

// Check is one check's answer.
type Check struct {
	Name       string `json:"name"`
	Answer     string `json:"answer"`               // yes | no
	Then       string `json:"then,omitempty"`       // change only: the opposite fact the tense suggests
	CanChange  string `json:"can_change,omitempty"` // change only
	Words      string `json:"words"`
	Why        string `json:"why"`
	Confidence string `json:"confidence"`
	Flagged    bool   `json:"flagged"`
	Call       string `json:"call"`
	Error      string `json:"error,omitempty"`
}

func schemaFor(name string) map[string]any {
	props := map[string]any{
		"answer":     map[string]any{"type": "string", "enum": []string{"yes", "no"}},
		"words":      map[string]any{"type": "string", "maxLength": 120},
		"why":        map[string]any{"type": "string", "maxLength": 160},
		"confidence": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
	}
	req := []string{"answer", "words", "why", "confidence"}
	if name == "change" {
		props["then"] = map[string]any{"type": "string", "maxLength": 160}
		props["can_change"] = map[string]any{"type": "string", "enum": []string{"yes", "no"}}
		req = []string{"answer", "then", "can_change", "words", "why", "confidence"}
	}
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}

// ask runs one check.
func ask(ctx context.Context, c *llm.Client, d checkDef, in Input, effort string) Check {
	out := Check{Name: d.Name}
	var sys, usr bytes.Buffer
	if err := tmpl.ExecuteTemplate(&sys, d.Name, in); err != nil {
		out.Error = err.Error()
		return out
	}
	if err := tmpl.ExecuteTemplate(&usr, "user", in); err != nil {
		out.Error = err.Error()
		return out
	}
	res, err := c.Chat(ctx, "judge/"+d.Name, Version, map[string]any{
		"reasoning_effort": effort,
		"temperature":      0.2,
		"seed":             1,
		"max_tokens":       2048,
		"messages":         []map[string]string{{"role": "system", "content": sys.String()}, {"role": "user", "content": usr.String()}},
		"response_format":  map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": d.Name, "strict": true, "schema": schemaFor(d.Name)}},
	})
	out.Call = res.CallID
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
		out.Error = "the model's answer didn't fit: " + err.Error()
	}
	out.Name, out.Call = d.Name, res.CallID
	out.Why, out.Words = strings.TrimSpace(out.Why), strings.TrimSpace(out.Words)
	switch {
	case out.Error != "":
	case out.Answer != "yes" && out.Answer != "no":
		out.Error = fmt.Sprintf("the model's answer didn't fit: answer %q", out.Answer)
	case d.Name == "change":
		out.Flagged = out.Answer == "yes" && out.CanChange == "no"
	default:
		out.Flagged = out.Answer == "yes"
	}
	return out
}

// Judge runs every check on one sample, in parallel, and combines them.
// The call id returned is the deciding check's.
func Judge(ctx context.Context, c *llm.Client, in Input, effort string) (Verdict, []Check, string, error) {
	checks := make([]Check, len(Checks))
	var wg sync.WaitGroup
	for i, d := range Checks {
		if d.Name == "change" && in.Kind == "action" { // only states hold over time (10/J2a)
			checks[i] = Check{Name: d.Name, Answer: "no", CanChange: "yes", Why: "not asked: actions don't hold over time", Confidence: "high"}
			continue
		}
		wg.Add(1)
		go func(i int, d checkDef) { defer wg.Done(); checks[i] = ask(ctx, c, d, in, effort) }(i, d)
	}
	wg.Wait()
	v, call, err := combine(checks)
	if call == "" { // a check that wasn't asked has no call
		for _, ch := range checks {
			if ch.Call != "" {
				call = ch.Call
				break
			}
		}
	}
	return v, checks, call, err
}

// combine is the verdict from the checks' answers (10/J3). Any failed check
// makes the whole answer unusable, since a missing ✕ would read as ✓.
func combine(checks []Check) (Verdict, string, error) {
	rank := map[string]int{"low": 0, "medium": 1, "high": 2}
	low := "high"
	for _, ch := range checks {
		if ch.Error != "" {
			return Verdict{}, ch.Call, fmt.Errorf("%s check: %s", ch.Name, ch.Error)
		}
	}
	for i, ch := range checks {
		if rank[ch.Confidence] < rank[low] {
			low = ch.Confidence
		}
		if ch.Flagged {
			d := Checks[i]
			return Verdict{Verdict: d.Verdict, Reason: d.Reason, ButKind: d.ButKind, Note: ch.Why, Confidence: ch.Confidence, Decided: d.Name}, ch.Call, nil
		}
	}
	return Verdict{Verdict: "ok", Confidence: low}, checks[0].Call, nil
}
