// Package judge is the model's first pass over samples (design/synthetic-
// data/10/M1): one call per sample, schema-constrained, validated. The prompt
// is a file; its hash is the prompt version (10/D113a).
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
	"text/template"

	"ganglion/internal/llm"
)

//go:embed prompts/judge.md
var promptFile string

var tmpl = template.Must(template.New("judge").Parse(promptFile))

// Version is the prompt file's hash, recorded with every call and verdict.
var Version = func() string { s := sha256.Sum256([]byte(promptFile)); return hex.EncodeToString(s[:])[:12] }()

// Input is everything the prompt shows for one sample.
type Input struct {
	Frame, Explainer string
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
}

var Reasons = []string{"Word Form", "Wrong Kind of Filler", "Ungrammatical", "Means Something Else", "Sounds Odd", "Other"}

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"verdict":    map[string]any{"type": "string", "enum": []string{"ok", "but", "bad"}},
		"but_kind":   map[string]any{"type": "string", "enum": []string{"", "context", "implies"}},
		"reason":     map[string]any{"type": "string", "enum": append([]string{""}, Reasons...)},
		"note":       map[string]any{"type": "string", "maxLength": 160},
		"confidence": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
	},
	"required":             []string{"verdict", "but_kind", "reason", "note", "confidence"},
	"additionalProperties": false,
}

// Judge asks the model about one sample. effort is gpt-oss's reasoning effort.
func Judge(ctx context.Context, c *llm.Client, in Input, effort string) (Verdict, string, error) {
	var sys, usr bytes.Buffer
	if err := tmpl.ExecuteTemplate(&sys, "system", in); err != nil {
		return Verdict{}, "", err
	}
	if err := tmpl.ExecuteTemplate(&usr, "user", in); err != nil {
		return Verdict{}, "", err
	}
	res, err := c.Chat(ctx, "judge", Version, map[string]any{
		"reasoning_effort": effort,
		"temperature":      0.2,
		"seed":             1,
		"max_tokens":       2048,
		"messages":         []map[string]string{{"role": "system", "content": sys.String()}, {"role": "user", "content": usr.String()}},
		"response_format":  map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "verdict", "strict": true, "schema": schema}},
	})
	if err != nil {
		return Verdict{}, res.CallID, err
	}
	var v Verdict
	if err := json.Unmarshal([]byte(res.Content), &v); err != nil {
		return Verdict{}, res.CallID, fmt.Errorf("the model's answer didn't fit: %w", err)
	}
	return v, res.CallID, validate(&v)
}

// validate makes the answer consistent, or says why it can't be used (10/G2).
func validate(v *Verdict) error {
	v.Note = strings.TrimSpace(v.Note)
	switch v.Verdict {
	case "ok":
		v.ButKind, v.Reason = "", ""
	case "but":
		v.Reason = ""
		if v.ButKind == "" {
			return fmt.Errorf("the model's answer didn't fit: ◐ without saying context or implies")
		}
	case "bad":
		v.ButKind = ""
		if v.Reason == "" {
			return fmt.Errorf("the model's answer didn't fit: ✕ without a reason")
		}
	default:
		return fmt.Errorf("the model's answer didn't fit: verdict %q", v.Verdict)
	}
	return nil
}
