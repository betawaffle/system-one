package web

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ganglion/internal/store"
)

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /static/", s.serveAsset)
	m.HandleFunc("GET /{$}", s.overview)
	m.HandleFunc("GET /events", s.events)
	m.HandleFunc("GET /events/{hash}", s.event)
	m.HandleFunc("GET /api/events/{hash}/lines", s.lines)
	m.HandleFunc("GET /fields", s.fields)
	m.HandleFunc("GET /utterances", s.utterances)
	m.HandleFunc("GET /utterances/{hash}", s.utterance)
	m.HandleFunc("GET /ettin", s.ettin)
	m.HandleFunc("GET /design", s.design)
	m.HandleFunc("GET /ws", s.ws)
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { s.fail(w, 404, "There's nothing at this address.") })
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	st := s.cfg.Store
	tot, err := st.Totals()
	if err != nil {
		s.fail(w, 500, err.Error())
		return
	}
	srcs, _ := st.SourceCounts()
	exits, exitTotal, _ := st.Stats("exit_code", 6)
	progs, _, _ := st.Stats("program", 12)
	lens, _, _ := st.Stats("out_len_log2", 40)
	fails := 0
	for _, e := range exits {
		if e.Value != "0" && e.Value != "none" {
			fails += e.Count
		}
	}
	if len(exits) == 6 {
		all, _, _ := st.Stats("exit_code", 1000)
		fails = 0
		for _, e := range all {
			if e.Value != "0" && e.Value != "none" {
				fails += e.Count
			}
		}
	}
	s.render(w, "overview", Page{Title: "Overview", Nav: "overview", Data: map[string]any{
		"Totals": tot, "Sources": srcs, "Exits": exits, "ExitTotal": exitTotal, "Fails": fails,
		"Programs": progs, "Lens": barsFor(lens), "Dup": tot.Occurrences - tot.Events,
	}})
}

// Bar is one row of a distribution, with its share for drawing.
type Bar struct {
	store.Stat
	W    string // width as a fraction of the largest, 0..1
	Bits float64
	Href string
}

func barsFor(st []store.Stat) []Bar {
	max, total := 0, 0
	for _, x := range st {
		total += x.Count
		if x.Count > max {
			max = x.Count
		}
	}
	out := make([]Bar, len(st))
	for i, x := range st {
		out[i] = Bar{Stat: x, W: strconv.FormatFloat(float64(x.Count)/float64(max), 'f', 4, 64)}
		if total > 0 {
			out[i].Bits = -math.Log2(float64(x.Count) / float64(total))
		}
	}
	return out
}

func filterFrom(v url.Values) store.Filter {
	return store.Filter{
		Q: strings.TrimSpace(v.Get("q")), Program: v.Get("program"), Exit: v.Get("exit"), Source: v.Get("source"),
		Split: v.Get("split"), Trunc: v.Get("truncated") == "1", Stderr: v.Get("stderr") == "1", Sort: v.Get("sort"),
	}
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	f := filterFrom(v)
	page, _ := strconv.Atoi(v.Get("page"))
	if page < 0 {
		page = 0
	}
	f.Limit = 50
	rows, more, err := s.cfg.Store.List(f, page*f.Limit)
	if err != nil {
		s.fail(w, 400, err.Error())
		return
	}
	count, _ := s.cfg.Store.Count(f)
	progs, _ := s.cfg.Store.Programs()
	srcs, _ := s.cfg.Store.SourceCounts()
	var srcNames []string
	seen := map[string]bool{}
	for _, c := range srcs {
		if !seen[c.Source] {
			seen[c.Source] = true
			srcNames = append(srcNames, c.Source)
		}
	}
	link := func(key, val string) string { return "/events" + query(v, key, val) }
	s.render(w, "events", Page{Title: "Events", Nav: "events", Data: map[string]any{
		"F": f, "Rows": rows, "Count": count, "Page": page, "More": more, "Programs": progs, "Sources": srcNames,
		"From": page*f.Limit + 1, "To": page*f.Limit + len(rows),
		"Prev": link("page", strconv.Itoa(page-1)), "Next": link("page", strconv.Itoa(page+1)),
		"Exit": func(val string) string { return link("exit", val) }, "Sort": func(val string) string { return link("sort", val) },
		"Split": func(val string) string { return link("split", val) },
		"Clear": func(key string) string { return link(key, "") },
	}})
}

// Section is one text field of an event, with its first lines rendered.
type Section struct {
	Name  string
	Field string
	Lines []string
	Total int
	Size  int
}

const firstLines = 60

func sectionOf(name, field string, text *string) *Section {
	if text == nil {
		return nil
	}
	lines := splitLines(*text)
	sec := &Section{Name: name, Field: field, Total: len(lines), Size: len(*text)}
	if len(lines) > firstLines {
		lines = lines[:firstLines]
	}
	sec.Lines = lines
	return sec
}

func splitLines(t string) []string {
	if t == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(t, "\n"), "\n")
}

func (s *Server) loadEvent(w http.ResponseWriter, hash string) (*store.Row, *store.Event, bool) {
	row, err := s.cfg.Store.Lookup(hash)
	if err != nil {
		s.fail(w, 404, "No event has that ID.")
		return nil, nil, false
	}
	ev, err := s.cfg.Store.Event(row.Hash)
	if err != nil {
		s.fail(w, 500, err.Error())
		return nil, nil, false
	}
	return row, ev, true
}

func (s *Server) event(w http.ResponseWriter, r *http.Request) {
	row, ev, ok := s.loadEvent(w, r.PathValue("hash"))
	if !ok {
		return
	}
	srcs, _ := s.cfg.Store.Sources(row.ID)
	type srcView struct {
		Source, Unit, Split string
		Turn                int
		Meta                map[string]any
	}
	var sv []srcView
	for _, x := range srcs {
		m := map[string]any{}
		json.Unmarshal(x.Meta, &m)
		sv = append(sv, srcView{x.Source, x.Unit, x.Split, x.Turn, m})
	}
	var secs []*Section
	for _, sec := range []*Section{sectionOf("Output", "output", ev.Output), sectionOf("Stdout", "stdout", ev.Stdout), sectionOf("Stderr", "stderr", ev.Stderr), sectionOf("Harness Error", "error", ev.Error)} {
		if sec != nil {
			secs = append(secs, sec)
		}
	}
	raw, _ := s.cfg.Store.Blob(row.Hash)
	s.render(w, "event", Page{Title: "Event " + row.Hash[:10], Nav: "events", Data: map[string]any{
		"Row": row, "Ev": ev, "Sections": secs, "Sources": sv, "RawSize": len(raw),
	}})
}

func (s *Server) lines(w http.ResponseWriter, r *http.Request) {
	row, err := s.cfg.Store.Lookup(r.PathValue("hash"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	ev, err := s.cfg.Store.Event(row.Hash)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var text *string
	switch r.URL.Query().Get("f") {
	case "output":
		text = ev.Output
	case "stdout":
		text = ev.Stdout
	case "stderr":
		text = ev.Stderr
	case "error":
		text = ev.Error
	}
	if text == nil {
		http.Error(w, "no such field", 400)
		return
	}
	lines := splitLines(*text)
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 2000 {
		n = 500
	}
	from = max(0, min(from, len(lines)))
	to := min(len(lines), from+n)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable") // content-addressed
	json.NewEncoder(w).Encode(map[string]any{"from": from, "lines": lines[from:to], "total": len(lines)})
}

func (s *Server) fields(w http.ResponseWriter, r *http.Request) {
	st := s.cfg.Store
	get := func(field string, n int, href func(string) string) ([]Bar, int) {
		x, total, _ := st.Stats(field, n)
		bars := barsFor(x)
		_, all, _ := st.Stats(field, 0)
		if all > 0 {
			for i := range bars {
				bars[i].Bits = -math.Log2(float64(bars[i].Count) / float64(all))
			}
		}
		for i := range bars {
			if href != nil {
				bars[i].Href = href(bars[i].Value)
			}
		}
		return bars, total
	}
	exits, exitTotal := get("exit_code", 24, func(v string) string {
		if v == "none" {
			return "/events?exit=none"
		}
		return "/events?exit=" + url.QueryEscape(v)
	})
	progs, _ := get("program", 30, func(v string) string { return "/events?program=" + url.QueryEscape(v) })
	lens, _ := get("out_len_log2", 40, nil)
	flags := map[string][2]int{}
	for _, f := range []string{"truncated", "error", "has_stderr"} {
		x, total, _ := st.Stats(f, 10)
		yes := 0
		for _, y := range x {
			if y.Value == "1" {
				yes = y.Count
			}
		}
		flags[f] = [2]int{yes, total}
	}
	tot, _ := st.Totals()
	s.render(w, "fields", Page{Title: "Fields", Nav: "fields", Data: map[string]any{
		"Exits": exits, "ExitTotal": exitTotal, "Programs": progs, "Lens": lens, "Flags": flags, "Totals": tot,
	}})
}

func (s *Server) utterances(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	f := store.UFilter{Q: strings.TrimSpace(v.Get("q")), Speaker: v.Get("speaker"), Kind: v.Get("kind"), Source: v.Get("source"), Split: v.Get("split"), Limit: 50}
	page, _ := strconv.Atoi(v.Get("page"))
	page = max(0, page)
	rows, more, total, err := s.cfg.Store.Utterances(f, page*f.Limit)
	if err != nil {
		s.fail(w, 400, err.Error())
		return
	}
	who, _ := s.cfg.Store.WhoCounts()
	srcs, _ := s.cfg.Store.USourceNames()
	kinds := map[string]bool{}
	var kindList []string
	for _, c := range who {
		if !kinds[c.Kind] {
			kinds[c.Kind] = true
			kindList = append(kindList, c.Kind)
		}
	}
	link := func(key, val string) string { return "/utterances" + query(v, key, val) }
	s.render(w, "utterances", Page{Title: "Utterances", Nav: "utterances", Data: map[string]any{
		"F": f, "Rows": rows, "Total": total, "Page": page, "More": more, "Who": who, "Sources": srcs, "Kinds": kindList,
		"From": page*f.Limit + 1, "To": page*f.Limit + len(rows),
		"Prev": link("page", strconv.Itoa(page-1)), "Next": link("page", strconv.Itoa(page+1)),
		"Speaker": func(val string) string { return link("speaker", val) },
	}})
}

func (s *Server) utterance(w http.ResponseWriter, r *http.Request) {
	row, u, srcs, err := s.cfg.Store.LookupUtterance(r.PathValue("hash"))
	if err != nil {
		s.fail(w, 404, "No utterance has that ID.")
		return
	}
	type srcView struct {
		Source, Unit, Split string
		Turn                int
		Meta                map[string]any
	}
	var sv []srcView
	for _, x := range srcs {
		m := map[string]any{}
		json.Unmarshal(x.Meta, &m)
		sv = append(sv, srcView{x.Source, x.Unit, x.Split, x.Turn, m})
	}
	s.render(w, "utterance", Page{Title: "Utterance " + row.Hash[:10], Nav: "utterances", Data: map[string]any{"Row": row, "U": u, "Sources": sv}})
}

func (s *Server) ettin(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	text := v.Get("text")
	var from string
	if h := v.Get("event"); h != "" {
		if row, err := s.cfg.Store.Lookup(h); err == nil {
			if ev, err := s.cfg.Store.Event(row.Hash); err == nil {
				from = row.Hash
				switch v.Get("f") {
				case "output":
					text = ev.MainText()
				case "stderr":
					text = derefStr(ev.Stderr)
				default:
					text = ev.Command
				}
			}
		}
	}
	if len(text) > 3000 {
		text = text[:3000]
	}
	mode := v.Get("mode")
	if mode == "" {
		mode = "surprise"
	}
	s.render(w, "ettin", Page{Title: "Ettin", Nav: "ettin", Data: map[string]any{"Text": text, "From": from, "Mode": mode}})
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) design(w http.ResponseWriter, r *http.Request) {
	s.render(w, "design", Page{Title: "Design", Nav: "design"})
}
