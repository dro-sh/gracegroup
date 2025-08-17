package gracegroup

import "time"

var DefaultShutdownTimeout = 5 * time.Second

type Config struct {
	// ShutdownTimeout is the maximum amount of time to wait for execution of all shutdown functions.
	// After this period, the shutdown process wont be waiting to finish.
	ShutdownTimeout time.Duration
}

var DefaultConfig = Config{
	ShutdownTimeout: DefaultShutdownTimeout,
}
