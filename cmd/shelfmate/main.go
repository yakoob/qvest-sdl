package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/httpapi"
	"school_district_reading/internal/store"
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
	fmt.Fprintf(os.Stderr, `shelfmate — librarian-in-the-loop next-book (Go skeleton)

  shelfmate recommend -student S-406 [-stretch] [-query "..."] [-data data/json]
  shelfmate serve -addr :8088 [-data data/json] [-web web]
  shelfmate eval

LLM is off unless SHELFMATE_LLM=on. Recs still work.
`)
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
	addr := fs.String("addr", ":8088", "listen address")
	data := fs.String("data", defaultData(), "json data dir")
	web := fs.String("web", defaultWeb(), "librarian console dir")
	_ = fs.Parse(args)

	st, err := store.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	eng := engine.New(st)
	srv := httpapi.Server{
		Engine: eng,
		Web:    http.FileServer(http.Dir(*web)),
	}
	log.Printf("shelfmate librarian console on http://127.0.0.1%s  students=%d books=%d", *addr, len(st.Students), len(st.Books))
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
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
