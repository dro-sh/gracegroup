# Gracegroup

Gracegroup is a package to execute processes and gracefully shutdown them. API is simple:

1. Add functions to start process and shutdown functions to stop process.
2. Wait for shutdown.

## Quickstart

Install **gracegroup**:

```bash
go get -u github.com/dro-sh/gracegroup
```

Simple example with http server with start and graceful shutdown:

```go
package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/dro-sh/gracegroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	srv := http.Server{}

	group, err := gracegroup.New(gracegroup.DefaultConfig)
	if err != nil {
		panic(err)
	}

	group.Add(srv.ListenAndServe, srv.Shutdown)

	if err := group.Wait(ctx); err != nil {
		panic(err)
	}
}
```

## Motivation

Almost all services contain a few executable processes and some of them need to be stopped gracefully, e.g, http server. So there is a need start and gracefully shutdown them. But each process should be start on its own goroutine and each shutdown must meet timeout for shutdown, and each process could initiate shutdown (on error, for example) and shutdown function should not affect to other shutdown functions. And this package indents to make it easy.

How it works:

- Add start and shutdown functions to `gracegroup.Group`.
- Wait while some of start function returns error on passed context will be canceled.
