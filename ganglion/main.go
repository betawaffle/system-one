// ganglion is the workbench for the system-one model project: browse the event
// corpus, play with Ettin, and later watch and steer training. See
// ../design/07-workbench.md and 08-workbench-design.md.
package main

import (
	"context"
	"embed"
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

	addr := flag.String("addr", ":7171", "listen address")
	data := flag.String("data", defaultData, "data directory")
	hosts := flag.String("hosts", os.Getenv("GANGLION_HOSTS"), "comma-separated hostnames to accept besides IPs and localhost (default $GANGLION_HOSTS)")
	sock := flag.String("worker", "", "worker socket (default <data>/worker.sock)")
	flag.Parse()
	if *sock == "" {
		*sock = filepath.Join(*data, "worker.sock")
	}

	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

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

// blacklistCmd: ganglion blacklist list | ganglion blacklist add HASH-PREFIX REASON [-by NAME]
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
