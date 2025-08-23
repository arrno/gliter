package gliter

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidOperationNotRunning error = errors.New("workerPool invalid operation error: workerPool is not running")
	ErrInvalidOperationRunning    error = errors.New("workerPool invalid operation error: workerPool is running")
	ErrInvalidOperationHasRun     error = errors.New("workerPool invalid operation error: workerPool has already run")
)

type WorkerConfig struct {
	size   int
	retry  int
	buffer int
}

type Option func(*WorkerConfig)

func WithRetry(attempts int) Option {
	return func(c *WorkerConfig) {
		c.retry = max(1, attempts)
	}
}

func WithBuffer(buffer int) Option {
	return func(c *WorkerConfig) {
		c.buffer = max(1, buffer)
	}
}

type WorkerPool[T, R any] struct {
	mu           sync.Mutex
	cfg          *WorkerConfig
	queue        chan T
	resultBuffer chan R
	handler      func(val T) (R, error)
	running      bool
	hasRun       bool
	results      []R
	wg           sync.WaitGroup
	errors       []error
}

func NewWorkerStage[T any](size int, handler func(val T) (T, error), opts ...Option) (
	stageFunc func([]T) ([]T, []error),
	close func() *WorkerPool[T, T],
) {
	workerPool := NewWorkerPool(size, handler, opts...)
	stageFunc = func(values []T) ([]T, []error) {
		workerPool.Push(values...)
		return workerPool.Drain()
	}
	close = workerPool.Close
	return
}

func NewWorkerPool[T, R any](size int, handler func(val T) (R, error), opts ...Option) *WorkerPool[T, R] {
	cfg := &WorkerConfig{size, 1, size * 2}
	for _, opt := range opts {
		opt(cfg)
	}
	wp := WorkerPool[T, R]{
		cfg:          cfg,
		queue:        make(chan T, cfg.buffer),
		resultBuffer: make(chan R, cfg.buffer),
		results:      make([]R, 0, cfg.buffer),
		handler:      handler,
	}
	wp.wg.Add(1)
	return wp.Boot()
}

func (b *WorkerPool[T, R]) Boot() *WorkerPool[T, R] {
	b.mu.Lock()
	if b.running || b.hasRun {
		b.mu.Unlock()
		return b
	}
	b.hasRun = true
	b.running = true
	b.mu.Unlock()

	var wg sync.WaitGroup

	for range b.cfg.size {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range b.queue {
				b.doWork(item)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(b.resultBuffer)
	}()

	go func() {
		cache := make([]R, 0, b.cfg.buffer)
		for result := range b.resultBuffer {
			cache = append(cache, result)
			if len(cache) >= b.cfg.buffer {
				b.mu.Lock()
				b.results = append(b.results, cache...)
				b.mu.Unlock()
				cache = make([]R, 0, b.cfg.buffer)
			}
		}
		b.mu.Lock()
		b.results = append(b.results, cache...)
		b.mu.Unlock()
		b.wg.Done()
	}()

	return b
}

func (b *WorkerPool[T, R]) doWork(task T) {
	for i := 0; i < b.cfg.retry; i++ {
		result, err := b.handler(task)
		if err != nil {
			b.mu.Lock()
			b.errors = append(b.errors, fmt.Errorf("work encountered err on attempt %d of %d. Err: %w", i+1, b.cfg.retry, err))
			b.mu.Unlock()
		} else {
			b.resultBuffer <- result
			break
		}
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
	for _, item := range items {
		b.queue <- item
	}
	return b
}

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

func (b *WorkerPool[T, R]) TakeErrors() (errors []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	errors = b.errors
	b.errors = nil
	return
}

func (b *WorkerPool[T, R]) TakeResults() (results []R) {
	b.mu.Lock()
	defer b.mu.Unlock()
	results = b.results
	b.results = nil
	return
}

func (b *WorkerPool[T, R]) Collect() (results []R, errors []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	results = b.results
	errors = b.errors
	b.results = nil
	b.errors = nil
	return
}

func (b *WorkerPool[T, R]) Drain() (results []R, errors []error) {
	// TODO this needs to force drain the work cache into results first
	b.mu.Lock()
	defer b.mu.Unlock()
	results = b.results
	errors = b.errors
	b.results = nil
	b.errors = nil
	return
}
