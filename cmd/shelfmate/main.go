package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/httpapi"
	"school_district_reading/internal/session"
	"school_district_reading/internal/store"
	"school_district_reading/internal/support"
	"school_district_reading/internal/version"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	switch cmd {
	case "recommend":
		recommend(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	case "eval":
		fmt.Fprintln(os.Stderr, "run: go test -v ./internal/eval")
		os.Exit(0)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %s\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `shelfmate — librarian-in-the-loop next-book (%s)

  shelfmate recommend -student S-406 [-stretch] [-query "..."] [-data data/json]
  shelfmate serve -addr 127.0.0.1:8088 [-data data/json] [-web web]
  shelfmate eval

LLM is off unless SHELFMATE_LLM=on. Recs still work.
Optional Axon: LLM_BASE (origin, client appends /v1/chat/completions),
LLM_MODEL, LLM_API_KEY (never logged).
Demo checkouts live in process memory; restart reloads the frozen extract.
`, version.Version)
}

func recommend(args []string) {
	fs := flag.NewFlagSet("recommend", flag.ExitOnError)
	student := fs.String("student", "S-406", "student_id")
	staff := fs.String("staff", "L-001", "staff_id")
	query := fs.String("query", "", "librarian natural-language intent")
	stretch := fs.Bool("stretch", false, "relax grade band one step")
	data := fs.String("data", defaultData(), "json data dir")
	_ = fs.Parse(args)

	st, err := store.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	eng := engine.New(st)
	rec, err := eng.Recommend(domain.Request{
		StudentID: *student,
		StaffID:   *staff,
		Query:     *query,
		Stretch:   *stretch,
		Limit:     5,
	})
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rec)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8088", "listen address (localhost by default)")
	data := fs.String("data", defaultData(), "json data dir")
	web := fs.String("web", defaultWeb(), "librarian console dir")
	_ = fs.Parse(args)

	listen := normalizeAddr(*addr)
	st, err := store.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	eng := engine.New(st)
	cat, err := academics.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	if err := cat.Validate(st); err != nil {
		log.Fatal(err)
	}
	if cat.Missing {
		log.Printf("academic_demo.json missing; reading pane will be empty")
	}
	guidance, err := support.Load(*data, st)
	if err != nil {
		log.Fatal(err)
	}
	svc := session.New(eng)
	srv := httpapi.Server{
		Support:   guidance,
		Session:   svc,
		Academics: cat,
		Web:       http.FileServer(http.Dir(*web)),
	}
	log.Printf("shelfmate librarian console on http://%s  students=%d books=%d version=%s  session=memory (restart resets demo checkouts)", displayURL(listen), len(st.Students), len(st.Books), version.Version)
	log.Fatal(http.ListenAndServe(listen, srv.Handler()))
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:8088"
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil && (host == "" || host == "0.0.0.0") {
		_, port, _ := net.SplitHostPort(addr)
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

func displayURL(addr string) string {
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:") {
		return addr
	}
	return addr
}

func defaultData() string {
	if v := os.Getenv("SHELFMATE_DATA"); v != "" {
		return v
	}
	candidates := []string{"data/json", filepath.Join(here(), "data/json")}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "catalog.json")); err == nil {
			return c
		}
	}
	return "data/json"
}

func defaultWeb() string {
	candidates := []string{"web", filepath.Join(here(), "web")}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "index.html")); err == nil {
			return c
		}
	}
	return "web"
}

func here() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}
