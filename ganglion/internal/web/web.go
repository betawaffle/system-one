// Package web serves ganglion's pages, static files and the playground socket.
package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"ganglion/internal/store"
	"ganglion/internal/worker"
)

type Config struct {
	Static    fs.FS
	Templates fs.FS
	Store     *store.Store
	Worker    *worker.Client
	Hosts     []string
	User      string // who labels come from: a name claim, no logins yet
}

type Server struct {
	cfg     Config
	mux     *http.ServeMux
	build   string // hash of all static files; part of every asset URL
	assets  map[string]asset
	pages   map[string]*template.Template
	handler http.Handler

	idxMu  sync.Mutex
	idxJob *indexJob
}

type asset struct {
	raw, gz []byte
	ctype   string
}

func New(cfg Config) (*Server, error) {
	s := &Server{cfg: cfg, mux: http.NewServeMux(), assets: map[string]asset{}, pages: map[string]*template.Template{}}
	if err := s.loadAssets(); err != nil {
		return nil, err
	}
	if err := s.loadTemplates(); err != nil {
		return nil, err
	}
	s.routes()
	s.handler = knownHost(cfg.Hosts, security(gzipMiddleware(s.mux)))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// loadAssets reads every static file, hashes the set, and precompresses text.
// Assets are served at /static/<build>/<path> with an immutable cache lifetime,
// so a browser fetches each one once per build (08/N1).
func (s *Server) loadAssets() error {
	h := sha256.New()
	var names []string
	fs.WalkDir(s.cfg.Static, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	for _, p := range names {
		b, err := fs.ReadFile(s.cfg.Static, p)
		if err != nil {
			return err
		}
		io.WriteString(h, p)
		h.Write(b)
		a := asset{raw: b, ctype: mime.TypeByExtension(path.Ext(p))}
		if a.ctype == "" {
			a.ctype = "application/octet-stream"
		}
		if !strings.HasSuffix(p, ".woff2") {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			zw.Write(b)
			zw.Close()
			if buf.Len() < len(b) {
				a.gz = buf.Bytes()
			}
		}
		s.assets[p] = a
	}
	s.build = hex.EncodeToString(h.Sum(nil))[:12]
	return nil
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/static/")
	build, p, ok := strings.Cut(rest, "/")
	a, found := s.assets[p]
	if !ok || !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", a.ctype)
	if build == s.build {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Vary", "Accept-Encoding")
	if a.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(a.gz)))
		w.Write(a.gz)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(a.raw)))
	w.Write(a.raw)
}

var pageFiles = []string{"overview", "events", "event", "fields", "utterances", "utterance", "samples", "ettin", "design", "error"}

func (s *Server) loadTemplates() error {
	funcs := template.FuncMap{
		"asset": func(p string) string { return "/static/" + s.build + "/" + p },
		"comma": comma,
		// title capitalises a fixed lowercase vocabulary word for a label ("human" -> "Human").
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"pct": func(a, b int) string {
			if b == 0 {
				return "0%"
			}
			v := 100 * float64(a) / float64(b)
			if v < 1 && v > 0 {
				return fmt.Sprintf("%.2f%%", v)
			}
			return fmt.Sprintf("%.1f%%", v)
		},
		"frac": func(a, b int) string {
			if b == 0 {
				return "0"
			}
			return strconv.FormatFloat(float64(a)/float64(b), 'f', 4, 64)
		},
		"bits": func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) },
		"short": func(h string) string {
			if len(h) > 10 {
				return h[:10]
			}
			return h
		},
		"bytes": humanBytes,
		"deref": func(p *int) int { return *p },
		"pow2":  func(s string) string { n, _ := strconv.Atoi(s); return humanBytes(1 << n) },
		"add":   func(a, b int) int { return a + b },
		"sub":   func(a, b int) int { return a - b },
		"pctf":  func(f float64) float64 { return 100 * f },
		"list":  func(xs ...string) []string { return xs },
		"split": func(s string) []string { return strings.SplitN(s, ":", 2) },
	}
	for _, name := range pageFiles {
		t, err := template.New("").Funcs(funcs).ParseFS(s.cfg.Templates, "layout.html", "parts.html", name+".html")
		if err != nil {
			return fmt.Errorf("template %s: %w", name, err)
		}
		s.pages[name] = t
	}
	return nil
}

// Page is the data every page template gets.
type Page struct {
	Title  string
	Nav    string
	Worker bool
	Data   any
}

func (s *Server) render(w http.ResponseWriter, name string, p Page) {
	p.Worker = s.cfg.Worker.Hello() != nil
	var buf bytes.Buffer
	if err := s.pages[name].ExecuteTemplate(&buf, "layout", p); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(buf.Bytes())
}

func (s *Server) fail(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	s.render(w, "error", Page{Title: http.StatusText(code), Data: map[string]any{"Code": code, "Msg": msg}})
}

func comma(n int) string {
	str := strconv.Itoa(n)
	neg := strings.HasPrefix(str, "-")
	str = strings.TrimPrefix(str, "-")
	var out []byte
	for i, c := range []byte(str) {
		if i > 0 && (len(str)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func humanBytes(n int) string {
	f := float64(n)
	for _, u := range []string{"B", "KB", "MB", "GB"} {
		if f < 1024 || u == "GB" {
			if u == "B" {
				return fmt.Sprintf("%d B", n)
			}
			if f < 10 {
				return fmt.Sprintf("%.1f %s", f, u)
			}
			return fmt.Sprintf("%.0f %s", math.Round(f), u)
		}
		f /= 1024
	}
	return ""
}

// knownHost refuses requests for hostnames it doesn't know, against DNS
// rebinding (a site pointing its own name at this machine). IP addresses and
// localhost can't be rebound to. Same approach as tack.
func knownHost(hosts []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		}
		host = strings.Trim(host, "[]")
		if net.ParseIP(host) == nil && !strings.EqualFold(host, "localhost") && !slices.ContainsFunc(hosts, func(k string) bool { return strings.EqualFold(strings.TrimSpace(k), host) }) {
			http.Error(w, "ganglion doesn't know this hostname; add it with -hosts.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// security sets headers on every response. Event text is model- and
// tool-produced, so it's only ever rendered as text; the policy is a second layer.
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type gzWriter struct {
	http.ResponseWriter
	zw      *gzip.Writer
	started bool
	skip    bool
}

func (g *gzWriter) WriteHeader(code int) {
	if !g.started {
		g.started = true
		h := g.Header()
		if h.Get("Content-Encoding") != "" || code == http.StatusNoContent || code == http.StatusNotModified {
			g.skip = true
		} else {
			h.Set("Content-Encoding", "gzip")
			h.Add("Vary", "Accept-Encoding")
			h.Del("Content-Length")
			g.zw = gzip.NewWriter(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzWriter) Write(b []byte) (int, error) {
	if !g.started {
		g.WriteHeader(http.StatusOK)
	}
	if g.skip {
		return g.ResponseWriter.Write(b)
	}
	return g.zw.Write(b)
}

// gzipMiddleware compresses dynamic responses (08/N7). Static assets are
// precompressed and the websocket is left alone.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" || strings.HasPrefix(r.URL.Path, "/static/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		g := &gzWriter{ResponseWriter: w}
		next.ServeHTTP(g, r)
		if g.zw != nil {
			g.zw.Close()
		}
	})
}

// query rebuilds a URL query with one key changed (empty removes it).
func query(v url.Values, key, val string) string {
	c := url.Values{}
	for k, vs := range v {
		c[k] = append([]string(nil), vs...)
	}
	if val == "" {
		c.Del(key)
	} else {
		c.Set(key, val)
	}
	c.Del("page")
	if key == "page" && val != "" {
		c.Set("page", val)
	}
	if len(c) == 0 {
		return "?"
	}
	return "?" + c.Encode()
}
