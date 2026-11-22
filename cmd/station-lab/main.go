//go:build linux

// station-lab has no production-device implementation or facility credentials.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deervery/raku-sika-hub/internal/station"
)

func main() {
	dir := flag.String("data", ".station-lab", "private durable mock state directory")
	port := flag.String("port", "3110", "loopback port")
	flag.Parse()
	s, err := station.Open(*dir, station.MockExecutor{})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	server := &http.Server{Addr: "127.0.0.1:" + *port, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
	}()
	log.Printf("MOCK ONLY station: http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-shutdownDone
}
