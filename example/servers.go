package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/dro-sh/gracegroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// some initialization code with db, logger, etc
	// that has defer functions to close connections, flush buffers, etc

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("pong")); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	})

	srv := http.Server{
		Addr:              ":8080",
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 1 * time.Second,
	}

	group := gracegroup.New(gracegroup.DefaultConfig)

	group.Add(srv.ListenAndServe, srv.Shutdown)

	if err := group.Wait(ctx); err != nil {
		panic(err) // dont fatal because upper could be defer functions
	}

	log.Println("server stopped")
}
