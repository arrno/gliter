package gliter

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidOperationNotRunning = errors.New("workerPool invalid operation error: workerPool is not running")
	ErrInvalidOperationRunning    = errors.New("workerPool invalid operation error: workerPool is running")
	ErrInvalidOperationHasRun     = errors.New("workerPool invalid operation error: workerPool has already run")
)

type WorkerConfig struct {
	size   int
	retry  int
	buffer int // used for input queue capacity
}

type Option func(*WorkerConfig)

func WithRetry(attempts int) Option {
	return func(c *WorkerConfig) { c.retry = max(1, attempts) }
}
func WithBuffer(buffer int) Option {
	return func(c *WorkerConfig) { c.buffer = max(1, buffer) }
}
func WithSize(size int) Option {
	return func(c *WorkerConfig) { c.size = max(1, size) }
}

type WorkerPool[T, R any] struct {
	mu      sync.Mutex
	cfg     *WorkerConfig
	queue   chan T
	handler func(T) (R, error)

	running bool
	hasRun  bool

	results []R
	errors  []error

	wg sync.WaitGroup // tracks worker goroutines
}

func NewWorkerPool[T, R any](size int, handler func(T) (R, error), opts ...Option) *WorkerPool[T, R] {
	cfg := &WorkerConfig{size: size, retry: 1, buffer: size * 2}
	for _, opt := range opts {
		opt(cfg)
	}
	return (&WorkerPool[T, R]{
		cfg:     cfg,
		queue:   make(chan T, cfg.buffer),
		results: make([]R, 0, cfg.buffer),
		handler: handler,
	}).Boot()
}

// Boot spawns workers.
func (b *WorkerPool[T, R]) Boot() *WorkerPool[T, R] {
	b.mu.Lock()
	if b.running || b.hasRun {
		b.mu.Unlock()
		return b
	}
	b.hasRun = true
	b.running = true
	b.mu.Unlock()

	for range b.cfg.size {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			for item := range b.queue {
				b.doWork(item)
			}
		}()
	}
	return b
}

func (b *WorkerPool[T, R]) doWork(task T) {
	for i := 0; i < b.cfg.retry; i++ {
		result, err := b.handler(task)
		if err != nil {
			b.mu.Lock()
			b.errors = append(b.errors, fmt.Errorf(
				"work encountered err on attempt %d of %d: %w", i+1, b.cfg.retry, err))
			b.mu.Unlock()
			continue
		}
		// Single critical section, fast append.
		b.mu.Lock()
		b.results = append(b.results, result)
		b.mu.Unlock()
		break
	}
}

func (b *WorkerPool[T, R]) Push(items ...T) *WorkerPool[T, R] {
	b.mu.Lock()
	if !b.running {
		b.errors = append(b.errors, ErrInvalidOperationNotRunning)
		b.mu.Unlock()
		return b
	}
	b.mu.Unlock()

	for _, it := range items {
		b.queue <- it
	}
	return b
}

// Close shuts down workers and waits for exit.
func (b *WorkerPool[T, R]) Close() *WorkerPool[T, R] {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return b
	}
	b.running = false
	b.mu.Unlock()

	close(b.queue)
	b.wg.Wait()
	return b
}

func (b *WorkerPool[T, R]) IsErr() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.errors) > 0
}

// TakeErrors clears and returns internal errors.
func (b *WorkerPool[T, R]) TakeErrors() (errs []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	errs = b.errors
	b.errors = nil
	return
}

// TakeResults clears and returns internal results.
func (b *WorkerPool[T, R]) TakeResults() (res []R) {
	b.mu.Lock()
	defer b.mu.Unlock()
	res = b.results
	b.results = nil
	return
}

// Collect clears and returns results and errors.
func (b *WorkerPool[T, R]) Collect() (res []R, errs []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	res = b.results
	errs = b.errors
	b.results = nil
	b.errors = nil
	return
}
