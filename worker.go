package gliter

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrInvalidOperationNotRunning error = errors.New("workerPool invalid operation error: workerPool is not running")
	ErrInvalidOperationRunning    error = errors.New("workerPool invalid operation error: workerPool is running")
	ErrInvalidOperationHasRun     error = errors.New("workerPool invalid operation error: workerPool has already run")
)

const DEFAULt_BUFF_SIZE = 100

type WorkerPool[T, R any] struct {
	mu           sync.Mutex
	size         int
	queue        chan T
	resultBuffer chan R
	bufferSize   int
	handler      func(val T) (R, error)
	running      bool
	hasRun       bool
	ctx          context.Context
	results      []R
	wg           sync.WaitGroup
	errors       []error
}

func NewWorkerPool[T, R any](ctx context.Context, size int, handler func(val T) (R, error)) *WorkerPool[T, R] {
	var wg sync.WaitGroup
	wg.Add(1)
	return &WorkerPool[T, R]{
		size:         size,
		queue:        make(chan T, size),
		bufferSize:   DEFAULt_BUFF_SIZE,
		resultBuffer: make(chan R, DEFAULt_BUFF_SIZE),
		results:      make([]R, 0, size),
		handler:      handler,
		wg:           wg,
		ctx:          ctx,
	}
}

func (b *WorkerPool[T, R]) WithBuffSize(buffSize int) error {
	if b.hasRun {
		return ErrInvalidOperationHasRun
	}
	b.bufferSize = buffSize
	b.resultBuffer = make(chan R, buffSize)
	return nil
}

func (b *WorkerPool[T, R]) Boot() {
	if b.running || b.hasRun {
		return
	}

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

	b.hasRun = true
	b.running = true
}

func (b *WorkerPool[T, R]) Push(items ...T) error {
	if !b.running {
		return ErrInvalidOperationNotRunning
	}
	for _, item := range items {
		b.queue <- item
	}
	return nil
}

func (b *WorkerPool[T, R]) Close() error {
	if !b.running {
		return ErrInvalidOperationNotRunning
	}

	b.running = false
	close(b.queue)
	b.wg.Wait()
	return nil
}

func (b *WorkerPool[T, R]) IsErr() bool {
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
