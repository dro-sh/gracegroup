package gracegroup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dro-sh/gracegroup"
)

func TestGroup(t *testing.T) {
	t.Parallel()

	errStartFn := errors.New("start Fn error")
	errShutdownFn := errors.New("shutdown Fn error")

	subtests := []struct {
		name          string
		cfg           gracegroup.Config
		shutdownAfter time.Duration
		startFns      []gracegroup.StartFn
		shutdownFns   []gracegroup.ShutdownFn
		expectedError error
	}{
		{
			name: "start func with error should stop group",
			cfg:  gracegroup.DefaultConfig,
			startFns: []gracegroup.StartFn{
				func() error { return errStartFn },
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return nil },
			},
			expectedError: errStartFn,
		},
		{
			name: "shutdown func returns error",
			cfg:  gracegroup.DefaultConfig,
			startFns: []gracegroup.StartFn{
				func() error { return nil },
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return errShutdownFn },
			},
			expectedError: errShutdownFn,
		},
		{
			name: "shutdown timeout must stop group if shutdown func running too long",
			cfg:  gracegroup.Config{ShutdownTimeout: 10 * time.Millisecond},
			startFns: []gracegroup.StartFn{
				func() error { return nil },
			},
			shutdownFns: []gracegroup.ShutdownFn{func(ctx context.Context) error {
				done := make(chan struct{})

				go func() {
					time.Sleep(250 * time.Millisecond)
					done <- struct{}{}
				}()

				select {
				case <-done:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}},
			expectedError: context.DeadlineExceeded,
		},
		{
			name: "one of start fuctions which returns nil error should not stop process",
			cfg:  gracegroup.DefaultConfig,
			// idea: if first function returns instantly nil, it should not stop process
			// then expected error should be from second function
			// that should mean that first function does not stop process
			startFns: []gracegroup.StartFn{
				func() error { return nil }, // function should not stop below function
				func() error {
					time.Sleep(250 * time.Millisecond)

					return errStartFn
				},
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			expectedError: errStartFn,
		},
		{
			name: "one of start fuctions which returns non-nil error should stop process",
			cfg:  gracegroup.DefaultConfig,
			// idea: if first function returns instantly error, it should stop process
			// then expected error should be from first function
			// that should mean that first function stops process
			startFns: []gracegroup.StartFn{
				func() error { return errStartFn },
				func() error {
					time.Sleep(4000 * time.Millisecond)

					return nil
				},
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			expectedError: errStartFn,
		},
		{
			name:          "canceled context passed to Wait func should stop process without error",
			cfg:           gracegroup.DefaultConfig,
			shutdownAfter: 250 * time.Millisecond,
			startFns: []gracegroup.StartFn{
				func() error {
					time.Sleep(5 * time.Second)

					return nil
				},
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return nil },
			},
			expectedError: nil,
		},
		{
			name: "all functions without errors should stop process without error",
			cfg:  gracegroup.DefaultConfig,
			startFns: []gracegroup.StartFn{
				func() error { return nil },
				func() error { return nil },
			},
			shutdownFns: []gracegroup.ShutdownFn{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			expectedError: nil,
		},
	}

	for _, subtest := range subtests {
		t.Run(subtest.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())

			// simulate canceled context
			if subtest.shutdownAfter > 0 {
				ctx, cancel = context.WithTimeoutCause(context.Background(), subtest.shutdownAfter, context.Canceled)
			}

			defer cancel()

			g := gracegroup.New(subtest.cfg)

			for i := range subtest.startFns {
				g.Add(subtest.startFns[i], subtest.shutdownFns[i])
			}

			if err := g.Wait(ctx); !errors.Is(err, subtest.expectedError) {
				t.Errorf("expected error %v, got %v", subtest.expectedError, err)
			}
		})
	}
}

func TestOneOfShutdownFunctionsReturnsError(t *testing.T) {
	t.Parallel()

	// idea: start1 sets worker1Running to true meaning that it is running
	// and shutdown1 sleeps and sets worker1Running to false
	// and shutdown2 instantly returns error.
	// shutdown2 should not affect shutdown1 and allow it to complete
	worker1Running := false

	// instantly return nil to initiate shutdown
	start1 := func() error {
		worker1Running = true

		return nil
	}

	start2 := func() error {
		return nil
	}

	shutdown1 := func(ctx context.Context) error {
		done := make(chan struct{})

		go func() {
			time.Sleep(250 * time.Millisecond)

			worker1Running = false
			done <- struct{}{}
		}()

		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}

	errShutdown2 := errors.New("shutdown error for second worker")

	shutdown2 := func(ctx context.Context) error {
		return errShutdown2
	}

	g := gracegroup.New(gracegroup.DefaultConfig)

	g.Add(start1, shutdown1)
	g.Add(start2, shutdown2)

	if err := g.Wait(context.Background()); !errors.Is(err, errShutdown2) {
		t.Errorf("expected error %v, got %v", errShutdown2, err)
	}

	if worker1Running {
		t.Errorf("worker1 should be stopped")
	}
}
