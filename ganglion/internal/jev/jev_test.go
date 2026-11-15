package jev

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequestShape(t *testing.T) {
	raw, err := Request(State{Sample: "Ana will be older than Lee.", Meaning: Meaning{Frame: "compare", Tense: "future", Roles: map[string]Role{"a": {FilledBy: "Ana"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		State     State                      `json:"state"`
		Model     string                     `json:"model"`
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(raw, &b); err != nil || b.Model != Model || b.State.Sample != "Ana will be older than Lee." || b.State.Meaning.Roles["a"].FilledBy != "Ana" {
		t.Fatalf("%v %+v", err, b)
	}
	for _, f := range flags {
		if !strings.Contains(string(b.Questions[f.q]), `"`+f.bad+`"`) {
			t.Errorf("question %s has no option %s", f.q, f.bad)
		}
	}
}

func TestCombine(t *testing.T) {
	as := map[string]answer{}
	for _, f := range flags {
		as[f.q] = answer{Choice: "fine-ish", Confidence: 0.9}
	}
	r, err := combine(as, "c")
	if err != nil || r.Verdict.Verdict != "ok" {
		t.Fatalf("%+v %v", r, err)
	}
	as["implies"] = answer{Choice: "implies_more", Confidence: 0.6}
	as["change"] = answer{Choice: "impossible_change", Confidence: 0.95}
	as["verdict"] = answer{Choice: "other", Confidence: 0.7}
	r, _ = combine(as, "c")
	if r.Verdict.Verdict != "bad" || r.Verdict.Decided != "change" || r.Direct.Reason != "Other" {
		t.Errorf("%+v", r)
	}
	delete(as, "grammar")
	if _, err := combine(as, "c"); err == nil {
		t.Error("a missing answer must fail")
	}
}
