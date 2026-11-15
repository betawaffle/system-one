// ganglion is the workbench for the system-one model project: browse the event
// corpus, play with Ettin, and later watch and steer training. See
// ../design/07-workbench.md and 08-workbench-design.md.
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ganglion/internal/author"
	"ganglion/internal/grammar"
	"ganglion/internal/lexicon"
	"ganglion/internal/llm"
	"ganglion/internal/store"
	"ganglion/internal/web"
	"ganglion/internal/worker"
)

//go:embed static templates
var files embed.FS

func main() {
	home, _ := os.UserHomeDir()
	defaultData := filepath.Join(home, ".local", "share", "ganglion")

	if len(os.Args) > 1 && os.Args[1] == "import" {
		importCmd(defaultData, os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "blacklist" {
		blacklistCmd(defaultData, os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "grammar" {
		grammarCmd(defaultData, os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "lexicon" {
		lexiconCmd(defaultData, os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "snapshot-db" {
		snapshotCmd(defaultData, os.Args[2:])
		return
	}

	addr := flag.String("addr", ":7171", "listen address")
	data := flag.String("data", defaultData, "data directory")
	hosts := flag.String("hosts", os.Getenv("GANGLION_HOSTS"), "comma-separated hostnames to accept besides IPs and localhost (default $GANGLION_HOSTS)")
	sock := flag.String("worker", "", "worker socket (default <data>/worker.sock)")
	user := flag.String("user", envOr("GANGLION_USER", "user"), "name labels are recorded under (a claim, not a login; default $GANGLION_USER)")
	llmURL := flag.String("llm", envOr("GANGLION_LLM", "http://localhost:8081"), "llama.cpp server for model features, or \"none\" (default $GANGLION_LLM)")
	llmModel := flag.String("llm-model", envOr("GANGLION_LLM_MODEL", "gpt-oss-120b"), "model id on that server")
	fewShot := flag.Bool("few-shot", os.Getenv("GANGLION_FEW_SHOT") == "1", "include the user's verdicts as examples in the judge prompt (off to start: design/synthetic-data/10/D113)")
	flag.Parse()
	if *sock == "" {
		*sock = filepath.Join(*data, "worker.sock")
	}

	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// the synthetic-data grammar (design/synthetic-data): files under <data>/grammar
	gram, err := grammar.Open(filepath.Join(*data, "grammar"))
	if err != nil {
		log.Fatalf("open grammar: %v", err)
	}
	if seeded, err := author.Seed(gram); err != nil {
		log.Fatalf("seed grammar: %v", err)
	} else if seeded {
		gram.AddEvent("seed", "ganglion", "ganglion/seed", map[string]any{"heads": len(gram.Heads(""))})
		log.Printf("seeded the grammar")
	}
	var lex *lexicon.Lexicon
	if p, err := lexicon.Latest(filepath.Join(gram.Dir(), "lexicon")); err == nil {
		if lex, err = lexicon.Load(p); err != nil {
			log.Fatalf("load lexicon: %v", err)
		}
		log.Printf("lexicon %s: %d entries", lex.Hash[:12], lex.Len())
	} else {
		log.Printf("no lexicon imported; the Write page needs `ganglion lexicon import`")
	}

	static, _ := fs.Sub(files, "static")
	tmpl, _ := fs.Sub(files, "templates")
	wk := worker.New(*sock)
	go wk.Run(context.Background())

	srv, err := web.New(web.Config{
		Static:    static,
		Templates: tmpl,
		Store:     st,
		Worker:    wk,
		Hosts:     strings.Split(*hosts, ","),
		User:      *user,
		Grammar:   gram,
		Lexicon:   lex,
		LLM:       llmClient(*llmURL, *llmModel, *data),
		FewShot:   *fewShot,
	})
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	hs := &http.Server{Addr: *addr, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(ctx)
	}()
	log.Printf("ganglion listening on %s", *addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// grammarCmd: ganglion grammar retire ID REASON [-by NAME]
// Retiring keeps every version (design/synthetic-data/07/V3); the reason is an
// authoring event linked from the retire record. Restart the service afterwards:
// it reads the grammar at start.
func grammarCmd(defaultData string, args []string) {
	fl := flag.NewFlagSet("grammar", flag.ExitOnError)
	data := fl.String("data", defaultData, "data directory")
	by := fl.String("by", "claude", "who is retiring it")
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && i+1 < len(args) {
			flags = append(flags, args[i], args[i+1])
			i++
		} else {
			pos = append(pos, args[i])
		}
	}
	fl.Parse(append(flags, pos...))
	if fl.Arg(0) != "retire" || fl.NArg() < 3 {
		log.Fatal("usage: ganglion grammar retire ID REASON [-by NAME]")
	}
	g, err := grammar.Open(filepath.Join(*data, "grammar"))
	if err != nil {
		log.Fatal(err)
	}
	id, reason := fl.Arg(1), strings.Join(fl.Args()[2:], " ")
	if _, ok := g.Head(id); !ok {
		log.Fatalf("%s has no current version", id)
	}
	ev, err := g.AddEvent("retire", *by, "ganglion/cli", map[string]any{"id": id, "reason": reason})
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Retire(id, grammar.Meta{By: *by, Client: "ganglion/cli", Event: ev}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("retired %s (event %s): %s\n", id, ev, reason)
}

// lexiconCmd: ganglion lexicon import [-data DIR] [-wordnet DIR] [-cmudict FILE]
// Builds a lexicon base file from the raw downloads (design/synthetic-data/05, 09/Z6).
func lexiconCmd(defaultData string, args []string) {
	if len(args) == 0 || args[0] != "import" {
		log.Fatal("usage: ganglion lexicon import [-data DIR] [-wordnet DIR] [-cmudict FILE]")
	}
	fl := flag.NewFlagSet("lexicon import", flag.ExitOnError)
	data := fl.String("data", defaultData, "data directory")
	wn := fl.String("wordnet", "", "WordNet dict directory (default <data>/raw/wordnet-3.1/dict)")
	cmu := fl.String("cmudict", "", "CMUdict file (default <data>/raw/cmudict/cmudict.dict)")
	fl.Parse(args[1:])
	if *wn == "" {
		*wn = filepath.Join(*data, "raw", "wordnet-3.1", "dict")
	}
	if *cmu == "" {
		*cmu = filepath.Join(*data, "raw", "cmudict", "cmudict.dict")
	}
	g, err := grammar.Open(filepath.Join(*data, "grammar"))
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	entries, err := lexicon.Build(lexicon.Sources{WordNetDict: *wn, CMUDict: *cmu})
	if err != nil {
		log.Fatal(err)
	}
	path, hash, err := lexicon.Write(filepath.Join(g.Dir(), "lexicon"), entries)
	if err != nil {
		log.Fatal(err)
	}
	doubt := map[string]int{}
	for _, e := range entries {
		doubt[e.POS] += min(len(e.Doubt), 1)
	}
	src := map[string]any{}
	for name, p := range map[string]string{"wordnet_index_noun": filepath.Join(*wn, "index.noun"), "wordnet_index_verb": filepath.Join(*wn, "index.verb"), "wordnet_index_adj": filepath.Join(*wn, "index.adj"), "wordnet_verb_exc": filepath.Join(*wn, "verb.exc"), "wordnet_noun_exc": filepath.Join(*wn, "noun.exc"), "wordnet_adj_exc": filepath.Join(*wn, "adj.exc"), "cmudict": *cmu} {
		b, err := os.ReadFile(p)
		if err != nil {
			log.Fatal(err)
		}
		sum := sha256.Sum256(b)
		src[name] = hex.EncodeToString(sum[:])
	}
	if _, err := g.AddEvent("lexicon-import", "ganglion", "ganglion/cli", map[string]any{"base": hash, "entries": len(entries), "doubt": doubt, "sources": src}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d entries (%d nouns, %d verbs, %d adjectives with a guessed form) in %s\n%s\n", len(entries), doubt["noun"], doubt["verb"], doubt["adj"], time.Since(start).Round(time.Millisecond), path)
}

// snapshotCmd: ganglion snapshot-db [-data DIR] [-out DIR]
// Writes consistent copies of the SQLite databases for the backup to pick up.
func snapshotCmd(defaultData string, args []string) {
	fl := flag.NewFlagSet("snapshot-db", flag.ExitOnError)
	data := fl.String("data", defaultData, "data directory")
	out := fl.String("out", "", "output directory (default <data>/snapshot)")
	fl.Parse(args)
	if *out == "" {
		*out = filepath.Join(*data, "snapshot")
	}
	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	start := time.Now()
	if err := st.Snapshot(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("snapshot written to %s in %s\n", *out, time.Since(start).Round(time.Millisecond))
}

// blacklistCmd:ganglion blacklist list | ganglion blacklist add HASH-PREFIX REASON [-by NAME]
func blacklistCmd(defaultData string, args []string) {
	fl := flag.NewFlagSet("blacklist", flag.ExitOnError)
	data := fl.String("data", defaultData, "data directory")
	by := fl.String("by", "claude", "who is adding the entry")
	// Go's flag package stops at the first positional argument; let flags go anywhere.
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && i+1 < len(args) {
			flags = append(flags, args[i], args[i+1])
			i++
		} else {
			pos = append(pos, args[i])
		}
	}
	fl.Parse(append(flags, pos...))
	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	switch fl.Arg(0) {
	case "list":
		bl, err := st.Blacklist()
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range bl {
			fmt.Printf("%s  %s  %s  (%s)\n", e.Hash[:12], e.Added, e.Reason, e.By)
		}
	case "add":
		if fl.NArg() < 3 {
			log.Fatal("usage: ganglion blacklist add HASH-PREFIX REASON")
		}
		e, row, err := st.AddToBlacklist(fl.Arg(1), strings.Join(fl.Args()[2:], " "), *by)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("blacklisted %s (%s %s, %d words): %s\n", e.Hash[:12], row.Speaker, row.Kind, row.Words, e.Reason)
	default:
		log.Fatal("usage: ganglion blacklist list | add HASH-PREFIX REASON")
	}
}

// importCmd reads NDJSON events (from the Python importers) on stdin.
func importCmd(defaultData string, args []string) {
	fl := flag.NewFlagSet("import", flag.ExitOnError)
	data := fl.String("data", defaultData, "data directory")
	fl.Parse(args)
	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	n, err := st.Import(os.Stdin, func(done int) {
		if done%5000 == 0 {
			fmt.Fprintf(os.Stderr, "\r%d events", done)
		}
	})
	fmt.Fprintf(os.Stderr, "\r%d events\n", n)
	if err != nil {
		log.Fatalf("import: %v", err)
	}
	if err := st.Analyze(); err != nil {
		log.Fatalf("stats: %v", err)
	}
}

func llmClient(url, model, data string) *llm.Client {
	if url == "" || url == "none" {
		return nil
	}
	return llm.New(url, model, filepath.Join(data, "model-calls"))
}
