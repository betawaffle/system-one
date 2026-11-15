package jev

import (
	"encoding/json"
	"testing"
)

func TestRequestShape(t *testing.T) {
	raw, err := Request(State{Sample: "Ana gave Lee a chair.", Meaning: map[string]string{"frame": "give", "giver": "Ana"}})
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		State     State                      `json:"state"`
		Model     string                     `json:"model"`
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(raw, &b); err != nil || b.Model != Model || b.State.Meaning["giver"] != "Ana" {
		t.Fatalf("%v %+v", err, b)
	}
	for _, n := range Names() {
		if b.Questions[n] == nil {
			t.Errorf("no question %s", n)
		}
	}
}

func TestCombine(t *testing.T) {
	as := map[string]answer{}
	for _, n := range Names() {
		as[n] = answer{Type: "noul", Noul: 0.95}
	}
	r, err := combine(as, "c")
	if err != nil || r.Verdict.Verdict != "ok" {
		t.Fatalf("%+v %v", r, err)
	}
	as["context"] = answer{Type: "noul", Noul: 0.2}
	as["meaning"] = answer{Type: "noul", Noul: 0.1}
	r, _ = combine(as, "c")
	if r.Verdict.Verdict != "bad" || r.Verdict.Decided != "meaning" || !r.Checks[3].Flagged {
		t.Errorf("%+v", r)
	}
	delete(as, "grammar")
	if _, err := combine(as, "c"); err == nil {
		t.Error("a missing answer must fail")
	}
}
