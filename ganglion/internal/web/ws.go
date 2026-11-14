package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// indexJob is a running similarity-index build; one at a time, shared by all pages.
type indexJob struct {
	Model, Field string
	Done, Total  int
}

var allowedOps = map[string]bool{"tokenize": true, "fill_mask": true, "surprise": true, "similarity": true, "nearest": true, "index_status": true, "load": true}

// ws is the playground socket. Browser messages are {id, op, ...}; replies carry
// the same id. The server also pushes {"push":"worker", ...} on worker changes and
// {"push":"index", ...} while an index builds. Frames are compressed (08/N6).
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer c.CloseNow()

	var wmu sync.Mutex
	send := func(v any) {
		b, _ := json.Marshal(v)
		wmu.Lock()
		defer wmu.Unlock()
		wctx, wc := context.WithTimeout(ctx, 10*time.Second)
		defer wc()
		c.Write(wctx, websocket.MessageText, b)
	}

	status := func() {
		h := s.cfg.Worker.Hello()
		s.idxMu.Lock()
		var job *indexJob
		if s.idxJob != nil {
			cp := *s.idxJob // copy under the lock; the build goroutine updates it
			job = &cp
		}
		s.idxMu.Unlock()
		send(map[string]any{"push": "worker", "up": h != nil, "hello": json.RawMessage(orNull(h)), "index": job})
	}
	watch, stop := s.cfg.Worker.Watch()
	defer stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-watch:
				status()
			}
		}
	}()
	status()

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var req map[string]any
		if json.Unmarshal(data, &req) != nil {
			continue
		}
		id := req["id"]
		op, _ := req["op"].(string)
		if op == "index_build" {
			go s.buildIndex(req, func(v map[string]any) { v["id"] = id; send(v) }, status)
			continue
		}
		if !allowedOps[op] {
			send(map[string]any{"id": id, "ok": false, "error": "unknown op"})
			continue
		}
		delete(req, "id")
		go func() {
			cctx, cc := context.WithTimeout(ctx, 2*time.Minute)
			defer cc()
			rep, err := s.cfg.Worker.Call(cctx, req, nil)
			if err != nil {
				send(map[string]any{"id": id, "ok": false, "error": err.Error()})
				return
			}
			var out map[string]any
			json.Unmarshal(rep, &out)
			out["id"] = id
			delete(out, "input_ids") // logged by the worker; not needed on the wire
			send(out)
		}()
	}
}

func orNull(b []byte) []byte {
	if b == nil {
		return []byte("null")
	}
	return b
}

// buildIndex embeds distinct commands for nearest-value search (08/K1d).
func (s *Server) buildIndex(req map[string]any, reply func(map[string]any), status func()) {
	model, _ := req["model"].(string)
	field := "command"
	s.idxMu.Lock()
	if s.idxJob != nil {
		s.idxMu.Unlock()
		reply(map[string]any{"ok": false, "error": "an index is already building"})
		return
	}
	job := &indexJob{Model: model, Field: field}
	s.idxJob = job
	s.idxMu.Unlock()
	defer func() {
		s.idxMu.Lock()
		s.idxJob = nil
		s.idxMu.Unlock()
		status()
	}()
	hashes, texts, err := s.cfg.Store.FieldTexts(field, 60000)
	if err != nil {
		reply(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.idxMu.Lock()
	job.Total = len(texts)
	s.idxMu.Unlock()
	status()
	last := time.Time{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	_, err = s.cfg.Worker.Call(ctx, map[string]any{"op": "index_build", "model": model, "field": field, "hashes": hashes, "texts": texts}, func(done, total int) {
		s.idxMu.Lock()
		job.Done = done
		s.idxMu.Unlock()
		if time.Since(last) > 400*time.Millisecond { // throttle pushes (08/N6)
			last = time.Now()
			status()
		}
	})
	if err != nil {
		log.Printf("index build: %v", err)
		reply(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	reply(map[string]any{"ok": true, "size": len(texts)})
}
