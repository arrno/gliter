package gliter

import (
	"errors"
	"sync"
)

var (
	ErrInvalidOperationNotRunning error = errors.New("workerPool invalid operation error: workerPool is not running")
	ErrInvalidOperationRunning    error = errors.New("workerPool invalid operation error: workerPool is running")
	ErrInvalidOperationHasRun     error = errors.New("workerPool invalid operation error: workerPool has already run")
)

type WorkerPool[T, R any] struct {
	mu           sync.Mutex
	size         int
	queue        chan T
	resultBuffer chan R
	bufferSize   int
	handler      func(val T) (R, error)
	running      bool
	hasRun       bool
	results      []R
	wg           sync.WaitGroup
	errors       []error
}

func NewWorkerPool[T, R any](size int, handler func(val T) (R, error)) *WorkerPool[T, R] {
	buffSize := size * 2
	wp := WorkerPool[T, R]{
		size:         size,
		queue:        make(chan T, buffSize),
		bufferSize:   buffSize,
		resultBuffer: make(chan R, buffSize),
		results:      make([]R, 0, buffSize),
		handler:      handler,
	}
	wp.wg.Add(1)
	return wp.boot()
}

func (b *WorkerPool[T, R]) boot() *WorkerPool[T, R] {
	b.mu.Lock()
	if b.running || b.hasRun {
		b.mu.Unlock()
		return b
	}
	b.hasRun = true
	b.running = true
	b.mu.Unlock()

	var wg sync.WaitGroup

	for range b.size {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range b.queue {
				r, err := b.handler(item)
				if err != nil {
					b.mu.Lock()
					b.errors = append(b.errors, err)
					b.mu.Unlock()
				} else {
					b.resultBuffer <- r
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(b.resultBuffer)
	}()

	go func() {
		cache := make([]R, 0, b.bufferSize)
		for result := range b.resultBuffer {
			cache = append(cache, result)
			if len(cache) >= b.bufferSize {
				b.mu.Lock()
				b.results = append(b.results, cache...)
				b.mu.Unlock()
				cache = make([]R, 0, b.bufferSize)
			}
		}
		b.mu.Lock()
		b.results = append(b.results, cache...)
		b.mu.Unlock()
		b.wg.Done()
	}()

	return b
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
