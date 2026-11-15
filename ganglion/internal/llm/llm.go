// Package llm is ganglion's one path to a language model (design/synthetic-
// data/07/D90, 10/C1-C3). Every call is logged raw and append-only, per the
// capture requirements: exact request and response bytes, timings, the model
// file's hash, and the server's build and chat template.
package llm

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Client struct {
	BaseURL string // e.g. http://localhost:8081
	Model   string // e.g. gpt-oss-120b
	Dir     string // <data>/model-calls
	http    *http.Client

	mu        sync.Mutex
	props     *serverInfo
	fileHash  map[string]string // model path+size+mtime -> sha256
	hashing   bool
	writeLock sync.Mutex
}

func New(baseURL, model, dir string) *Client {
	return &Client{BaseURL: baseURL, Model: model, Dir: dir, http: &http.Client{Timeout: 5 * time.Minute}, fileHash: map[string]string{}}
}

type serverInfo struct {
	BuildInfo        string `json:"build_info"`
	ModelPath        string `json:"model_path"`
	ChatTemplateHash string `json:"chat_template_sha256"`
	TotalSlots       int    `json:"total_slots"`
}

// Record is one line of model-calls/YYYY-MM.jsonl.
type Record struct {
	Schema    int               `json:"schema"`
	ID        string            `json:"id"`
	Provider  string            `json:"provider,omitempty"` // empty: the local llm-server; "typesafe": Jev
	Purpose   string            `json:"purpose"`            // judge, span, helper, rules…
	Prompt    string            `json:"prompt,omitempty"`   // prompt file version (sha256)
	Client    string            `json:"client"`             // ganglion and its build
	URL       string            `json:"url"`
	Requested string            `json:"model_requested"`
	Returned  string            `json:"model_returned,omitempty"`
	ModelFile map[string]any    `json:"model_file,omitempty"` // path, size, mtime, sha256 (once known)
	Server    *serverInfo       `json:"server,omitempty"`
	Sent      string            `json:"at_sent"`
	FirstByte string            `json:"at_first_byte,omitempty"`
	Done      string            `json:"at_done"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"response_headers,omitempty"`
	Request   json.RawMessage   `json:"request"`            // exact bytes sent
	Response  string            `json:"response,omitempty"` // exact bytes received
	Error     string            `json:"error,omitempty"`
	RetryOf   string            `json:"retry_of,omitempty"`
}

// Build identifies this ganglion binary in records (set by main).
var Build = "dev"

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// Status says whether the server answers and whether the model is loaded.
func (c *Client) Status(ctx context.Context) (up bool, state string) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/v1/models", nil)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return false, "down"
	}
	defer resp.Body.Close()
	var d struct {
		Data []struct {
			ID     string `json:"id"`
			Status struct {
				Value string `json:"value"`
			} `json:"status"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&d)
	for _, m := range d.Data {
		if m.ID == c.Model {
			return true, m.Status.Value
		}
	}
	return true, "unknown"
}

// info fetches the server's build and chat template once per load; the
// template itself is stored once, by hash, beside the call logs.
func (c *Client) info(ctx context.Context) *serverInfo {
	c.mu.Lock()
	if c.props != nil {
		defer c.mu.Unlock()
		return c.props
	}
	c.mu.Unlock()
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/props?model="+c.Model, nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var p struct {
		BuildInfo    string `json:"build_info"`
		ModelPath    string `json:"model_path"`
		ChatTemplate string `json:"chat_template"`
		TotalSlots   int    `json:"total_slots"`
	}
	if json.NewDecoder(resp.Body).Decode(&p) != nil || p.BuildInfo == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(p.ChatTemplate))
	h := hex.EncodeToString(sum[:])
	os.MkdirAll(filepath.Join(c.Dir, "templates"), 0o755)
	tp := filepath.Join(c.Dir, "templates", h+".txt")
	if _, err := os.Stat(tp); err != nil {
		os.WriteFile(tp, []byte(p.ChatTemplate), 0o644)
	}
	si := &serverInfo{BuildInfo: p.BuildInfo, ModelPath: p.ModelPath, ChatTemplateHash: h, TotalSlots: p.TotalSlots}
	c.mu.Lock()
	c.props = si
	c.mu.Unlock()
	go c.hashModelFile(p.ModelPath)
	return si
}

// hashModelFile computes the model file's sha256 once (a 60 GB read), caching
// it by path, size and modification time in model-files.json.
func (c *Client) hashModelFile(path string) {
	st, err := os.Stat(path)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s|%d|%d", path, st.Size(), st.ModTime().Unix())
	cache := filepath.Join(c.Dir, "model-files.json")
	c.mu.Lock()
	if c.hashing || c.fileHash[key] != "" {
		c.mu.Unlock()
		return
	}
	known := map[string]string{}
	if b, err := os.ReadFile(cache); err == nil {
		json.Unmarshal(b, &known)
	}
	if h := known[key]; h != "" {
		c.fileHash[key] = h
		c.mu.Unlock()
		return
	}
	c.hashing = true
	c.mu.Unlock()
	f, err := os.Open(path)
	if err == nil {
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			known[key] = hex.EncodeToString(h.Sum(nil))
			b, _ := json.MarshalIndent(known, "", "  ")
			os.WriteFile(cache, b, 0o644)
		}
		f.Close()
	}
	c.mu.Lock()
	c.fileHash[key] = known[key]
	c.hashing = false
	c.mu.Unlock()
}

func (c *Client) modelFile(si *serverInfo) map[string]any {
	if si == nil {
		return nil
	}
	st, err := os.Stat(si.ModelPath)
	if err != nil {
		return map[string]any{"path": si.ModelPath}
	}
	key := fmt.Sprintf("%s|%d|%d", si.ModelPath, st.Size(), st.ModTime().Unix())
	c.mu.Lock()
	h := c.fileHash[key]
	c.mu.Unlock()
	out := map[string]any{"path": si.ModelPath, "size": st.Size(), "mtime": st.ModTime().UTC().Format(time.RFC3339)}
	if h != "" {
		out["sha256"] = h
	}
	return out
}

// Result is a chat completion's useful parts plus the call's id.
type Result struct {
	CallID    string
	Content   string
	Reasoning string
	Model     string
}

// Chat sends a chat-completions request and logs it. body must not set model;
// the client does.
func (c *Client) Chat(ctx context.Context, purpose, prompt string, body map[string]any) (Result, error) {
	body["model"] = c.Model
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	si := c.info(ctx)
	rec := Record{Schema: 1, ID: newID(), Purpose: purpose, Prompt: prompt, Client: "ganglion " + Build, URL: c.BaseURL + "/v1/chat/completions",
		Requested: c.Model, Request: raw, Server: si, ModelFile: c.modelFile(si), Sent: now()}
	req, _ := http.NewRequestWithContext(ctx, "POST", rec.URL, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		rec.Done, rec.Error = now(), err.Error()
		c.log(rec)
		return Result{CallID: rec.ID}, err
	}
	rec.FirstByte = now() // headers arrived; the body follows
	rec.Status = resp.StatusCode
	rec.Headers = map[string]string{}
	for k, v := range resp.Header {
		if len(v) > 0 {
			rec.Headers[k] = v[0]
		}
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	rec.Done, rec.Response = now(), string(b)
	if err != nil {
		rec.Error = err.Error()
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err == nil && resp.StatusCode == 200 {
		if e := json.Unmarshal(b, &out); e != nil {
			rec.Error = "decode: " + e.Error()
		}
	} else if err == nil {
		rec.Error = fmt.Sprintf("status %d", resp.StatusCode)
	}
	rec.Returned = out.Model
	if lerr := c.log(rec); lerr != nil {
		return Result{CallID: rec.ID}, lerr // never use an answer we failed to record
	}
	if rec.Error != "" {
		return Result{CallID: rec.ID}, errors.New(rec.Error)
	}
	if len(out.Choices) == 0 {
		return Result{CallID: rec.ID}, errors.New("no choices")
	}
	m := out.Choices[0].Message
	return Result{CallID: rec.ID, Content: m.Content, Reasoning: m.ReasoningContent, Model: out.Model}, nil
}

func (c *Client) log(rec Record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	f, err := os.OpenFile(filepath.Join(c.Dir, time.Now().UTC().Format("2006-01")+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// Find returns one logged call by id, searching the monthly files newest first.
func (c *Client) Find(id string) (*Record, error) {
	paths, _ := filepath.Glob(filepath.Join(c.Dir, "*.jsonl"))
	for i := len(paths) - 1; i >= 0; i-- {
		b, err := os.ReadFile(paths[i])
		if err != nil {
			continue
		}
		needle := []byte(`"id":"` + id + `"`)
		for _, line := range bytes.Split(b, []byte("\n")) {
			if !bytes.Contains(line, needle) {
				continue
			}
			var r Record
			if json.Unmarshal(line, &r) == nil && r.ID == id {
				return &r, nil
			}
		}
	}
	return nil, os.ErrNotExist
}

// Post sends a JSON request to another provider's endpoint and logs it the
// same way as Chat. The key goes in the Authorization header only; it is never
// recorded (capture requirements 28).
func Post(ctx context.Context, dir, provider, purpose, prompt, url, key, model string, raw []byte) (*Record, error) {
	rec := Record{Schema: 1, ID: newID(), Provider: provider, Purpose: purpose, Prompt: prompt, Client: "ganglion " + Build, URL: url,
		Requested: model, Request: raw, Sent: now()}
	c := &Client{Dir: dir}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(raw))
	if err != nil {
		return &rec, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		rec.Done, rec.Error = now(), err.Error()
		c.log(rec)
		return &rec, err
	}
	rec.FirstByte = now()
	rec.Status = resp.StatusCode
	rec.Headers = map[string]string{}
	for k, v := range resp.Header {
		if len(v) > 0 && k != "Set-Cookie" {
			rec.Headers[k] = v[0]
		}
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	rec.Done, rec.Response = now(), string(b)
	if err != nil {
		rec.Error = err.Error()
	} else if resp.StatusCode != 200 {
		rec.Error = fmt.Sprintf("status %d", resp.StatusCode)
	}
	var out struct {
		Model string `json:"model"`
	}
	json.Unmarshal(b, &out)
	rec.Returned = out.Model
	if lerr := c.log(rec); lerr != nil {
		return &rec, lerr
	}
	if rec.Error != "" {
		return &rec, errors.New(rec.Error)
	}
	return &rec, nil
}
