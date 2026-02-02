package runtime

import "time"

// --- Start Options ---

type startOptions struct {
	Detach bool
	Remove bool
	Env    map[string]string
}

// StartOption configures container start behavior.
type StartOption func(*startOptions)

func defaultStartOptions() *startOptions {
	return &startOptions{
		Detach: true,
		Remove: false,
		Env:    make(map[string]string),
	}
}

func applyStartOptions(opts []StartOption) *startOptions {
	o := defaultStartOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithDetach sets whether the container runs in detached mode.
func WithDetach(detach bool) StartOption {
	return func(o *startOptions) {
		o.Detach = detach
	}
}

// WithRemoveOnExit sets whether the container is removed after exit.
func WithRemoveOnExit(remove bool) StartOption {
	return func(o *startOptions) {
		o.Remove = remove
	}
}

// WithEnv adds environment variables for the container start.
func WithEnv(env map[string]string) StartOption {
	return func(o *startOptions) {
		for k, v := range env {
			o.Env[k] = v
		}
	}
}

// --- Stop Options ---

type stopOptions struct {
	Timeout time.Duration
}

// StopOption configures container stop behavior.
type StopOption func(*stopOptions)

func defaultStopOptions() *stopOptions {
	return &stopOptions{
		Timeout: 10 * time.Second,
	}
}

func applyStopOptions(opts []StopOption) *stopOptions {
	o := defaultStopOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithStopTimeout sets the timeout before force-killing the container.
func WithStopTimeout(d time.Duration) StopOption {
	return func(o *stopOptions) {
		o.Timeout = d
	}
}

// --- Remove Options ---

type removeOptions struct {
	Force   bool
	Volumes bool
}

// RemoveOption configures container removal behavior.
type RemoveOption func(*removeOptions)

func defaultRemoveOptions() *removeOptions {
	return &removeOptions{
		Force:   false,
		Volumes: false,
	}
}

func applyRemoveOptions(opts []RemoveOption) *removeOptions {
	o := defaultRemoveOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithForceRemove forces removal of a running container.
func WithForceRemove(force bool) RemoveOption {
	return func(o *removeOptions) {
		o.Force = force
	}
}

// WithRemoveVolumes removes associated anonymous volumes.
func WithRemoveVolumes(volumes bool) RemoveOption {
	return func(o *removeOptions) {
		o.Volumes = volumes
	}
}

// --- Log Options ---

type logOptions struct {
	Follow bool
	Since  string
	Until  string
	Tail   string
}

// LogOption configures log retrieval behavior.
type LogOption func(*logOptions)

func defaultLogOptions() *logOptions {
	return &logOptions{
		Follow: false,
		Since:  "",
		Until:  "",
		Tail:   "all",
	}
}

func applyLogOptions(opts []LogOption) *logOptions {
	o := defaultLogOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithFollow enables streaming of new log output.
func WithFollow(follow bool) LogOption {
	return func(o *logOptions) {
		o.Follow = follow
	}
}

// WithSince returns logs newer than the given timestamp or duration.
func WithSince(since string) LogOption {
	return func(o *logOptions) {
		o.Since = since
	}
}

// WithUntil returns logs older than the given timestamp or duration.
func WithUntil(until string) LogOption {
	return func(o *logOptions) {
		o.Until = until
	}
}

// WithTail returns the specified number of lines from the end.
// Use "all" for all lines or a numeric string like "100".
func WithTail(tail string) LogOption {
	return func(o *logOptions) {
		o.Tail = tail
	}
}
