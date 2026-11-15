package judge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"bytes"
	"ganglion/internal/llm"
	"strings"
	"testing"
)

func TestEveryCheckRenders(t *testing.T) {
	in := Input{Frame: "Has Property", Explainer: "x", Roles: []string{"holder: apple"}, Authored: "This apple is old", Sample: "That apple was old.", Tags: "past", Notes: []string{"implies refusal"}}
	for _, d := range Checks {
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, d.Name, in); err != nil {
			t.Fatalf("%s: %v", d.Name, err)
		}
		if !strings.Contains(b.String(), "Question:") {
			t.Errorf("%s: no question", d.Name)
		}
		if has := strings.Contains(b.String(), "implies refusal"); has != (d.Name == "implies" || d.Name == "context") {
			t.Errorf("%s: notes shown = %v (10/J4)", d.Name, has)
		}
	}
}

func TestCombine(t *testing.T) {
	no := func(n string) Check { return Check{Name: n, Answer: "no", Confidence: "high", Call: n} }
	all := func(mod func([]Check)) []Check {
		var cs []Check
		for _, d := range Checks {
			cs = append(cs, no(d.Name))
		}
		mod(cs)
		return cs
	}
	v, _, err := combine(all(func(cs []Check) { cs[6].Confidence = "low" }))
	if err != nil || v.Verdict != "ok" || v.Confidence != "low" {
		t.Errorf("all no: %+v %v", v, err)
	}
	// ✕ beats ◐, and the earlier ✕ wins
	v, call, _ := combine(all(func(cs []Check) {
		cs[5].Flagged = true
		cs[3].Flagged, cs[3].Why = true, "adds a gift"
		cs[4].Flagged = true
	}))
	if v.Verdict != "bad" || v.Reason != "Means Something Else" || v.Note != "adds a gift" || call != "meaning" {
		t.Errorf("first ✕: %+v %s", v, call)
	}
	v, _, _ = combine(all(func(cs []Check) { cs[6].Flagged = true }))
	if v.Verdict != "but" || v.ButKind != "context" {
		t.Errorf("context: %+v", v)
	}
	if _, _, err := combine(all(func(cs []Check) { cs[2].Error = "timeout" })); err == nil {
		t.Error("a failed check must fail the verdict")
	}
}

func TestActionSkipsChange(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			n.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"m","choices":[{"message":{"content":"{\"answer\":\"no\",\"words\":\"\",\"why\":\"\",\"confidence\":\"high\"}"}}]}`))
	}))
	defer srv.Close()
	c := llm.New(srv.URL, "m", t.TempDir())
	v, checks, call, err := Judge(context.Background(), c, Input{Kind: "action", Sample: "Ana gave Lee a chair."}, "low")
	if err != nil || v.Verdict != "ok" || call == "" {
		t.Fatalf("%+v %q %v", v, call, err)
	}
	if n.Load() != int32(len(Checks)-1) || checks[4].Name != "change" || checks[4].Call != "" {
		t.Errorf("asked %d checks, change = %+v", n.Load(), checks[4])
	}
}
