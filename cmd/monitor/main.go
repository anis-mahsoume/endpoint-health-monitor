// Command monitor checks a set of HTTP endpoints on a fixed interval and serves
// their status through a JSON API and a web dashboard.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/scheduler"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/server"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	workers := flag.Int("workers", 10, "number of concurrent checks")
	timeout := flag.Duration("timeout", 5*time.Second, "per-check timeout")
	interval := flag.Duration("interval", 10*time.Second, "time between check rounds")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c := checker.New(*timeout)
	st := store.New(20, 3)

	endpoints := []checker.Endpoint{
		{ID: "Github", URL: "https://github.com"},
		{ID: "Google", URL: "https://www.google.com"},
		{ID: "Notfound", URL: "https://github.com/this-does-not-exist-xyz"},
		{ID: "Broken", URL: "https://nothing.invalid"},
		{ID: "Closed-port", URL: "http://localhost:1"},
		{ID: "Hang", URL: "http://10.255.255.1"},
	}
	for _, ep := range endpoints {
		if err := st.Add(ep); err != nil {
			log.Fatalf("add %s: %v", ep.ID, err)
		}
	}

	sched := scheduler.New(c, st, *interval, *workers)

	schedDone := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(schedDone)
	}()

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Println("[WARNING] API_KEY not set, POST and DELETE are unprotected")
	}

	srv := &http.Server{Addr: *addr, Handler: server.New(st, apiKey)}

	go func() {
		log.Printf("listening on %s (dashboard at /, API at /api/v1)", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	log.Println("waiting for the current check round to finish...")
	<-schedDone
	log.Println("stopped")
}
