package gracegroup

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

type (
	// StartFn does not accept context as argument because most of start functions do not need it
	// because then context canceled shutdown functions should be called.
	// If start fn returns error then group will be shutdown, otherwise it won't.
	StartFn func() error
	// ShutdownFn accepts context as argument to handle shutdown timeout and must
	// support handling timed out context.
	// If function returns error it wont affect to other shutdown functions.
	ShutdownFn func(ctx context.Context) error
)

// Gracegroup is a managare to execute processes and functions to shutdown processes.
type Group struct {
	mu sync.Mutex

	cfg         Config
	startFns    []StartFn
	shutdownFns []ShutdownFn
}

func New(cfg Config) *Group {
	return &Group{
		cfg:         cfg,
		startFns:    make([]StartFn, 0),
		shutdownFns: make([]ShutdownFn, 0),
	}
}

// Add adds a start function and a shutdown function to the group.
// Func does not invoke start func immediately, it will wait for Wait method.
func (r *Group) Add(start StartFn, shutdown ShutdownFn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.startFns = append(r.startFns, start)
	r.shutdownFns = append(r.shutdownFns, shutdown)
}

// Wait does next things:
//  1. invokes all start functions concurrently,
//  2. waits one of next condition:
//     a. passed to Wait context is canceled,
//     b. one of start functions returns error,
//  3. invokes all shutdown functions concurrently and waiting while all of them will be finished.
//
// If any of start functions return nil then group won't initiate shutdown.
// If any of shutdown functions returns error it wont stop shutdown process. Shutdown function
// must support context DeadlineExceeded error and exit on it. Group does not forcelly stop shutdown
// then context deadline exceeded.
//
// Wait could return error from:
//  1. one of start functions,
//  2. one of shutdown functions,
//  3. error from Wait context if it is not context.Calceled error,
//  4. context.DeadlineExceeded if cfg.ShutdownTimeout is exceeded on shutdown.
func (r *Group) Wait(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, start := range r.startFns {
		g.Go(start)
	}

	err := r.wait(ctx, g)

	shutdownError := r.shutdown()

	return errors.Join(shutdownError, err)
}

func (r *Group) wait(ctx context.Context, g *errgroup.Group) error {
	done := make(chan struct{})

	go func() {
		//nolint:errcheck,gosec // err will be set on errgroup context cause
		g.Wait()

		close(done)
	}()

	// no needs set error from ctx or errgroup
	// because errgroup set error cause to context on wait method
	// or argument context has error cause
	select {
	case <-done:
	case <-ctx.Done():
	}

	if err := context.Cause(ctx); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (r *Group) shutdown() error {
	ctx, cancel := context.WithCancel(context.Background())

	if r.cfg.ShutdownTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, r.cfg.ShutdownTimeout)
	}
	defer cancel()

	g := &errgroup.Group{}

	for _, shutdownFn := range r.shutdownFns {
		g.Go(func() error {
			return shutdownFn(ctx)
		})
	}

	return g.Wait()
}
