// Command task033-bizcal runs the business-calendar service.
//
// Use --smoke-test to run the built-in self-check, which exits the process on
// completion. Otherwise it serves the HTTP API with `server --addr :8080`,
// loading the calendar definition from the JSON file named by --calendar
// (default calendar.json in the working directory).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"task033-bizcal/internal/calendar"
	"task033-bizcal/internal/httpapi"
	"task033-bizcal/internal/selfcheck"
)

func main() {
	args := os.Args[1:]

	// --smoke-test runs the self-check and exits.
	if len(args) > 0 && args[0] == "--smoke-test" {
		if err := selfcheck.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "smoke-test FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("smoke-test PASSED")
		return
	}

	// Tolerate an optional leading "server" subcommand so flags after it
	// (e.g. `server --addr :9090`) are parsed by the flag set.
	if len(args) > 0 && args[0] == "server" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("bizcal", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	calPath := fs.String("calendar", "calendar.json", "calendar config JSON path")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s --smoke-test                 run self-check and exit\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s server --addr :8080          start the HTTP server\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cal, cfg, err := loadCalendar(*calPath)
	if err != nil {
		log.Fatalf("init calendar: %v", err)
	}

	srv := httpapi.New(cal)
	hs := &http.Server{Addr: *addr, Handler: srv.Handler()}
	log.Printf("task033-bizcal listening on %s (timezone=%s)", *addr, cfg.Timezone)
	if err := hs.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadCalendar reads and validates the calendar config file.
func loadCalendar(path string) (*calendar.Calendar, calendar.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, calendar.Config{}, fmt.Errorf("read %q: %w", path, err)
	}
	var cfg calendar.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, cfg, fmt.Errorf("parse config: %w", err)
	}
	cal, err := calendar.New(cfg)
	if err != nil {
		return nil, cfg, err
	}
	return cal, cfg, nil
}
