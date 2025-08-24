package gliter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	p := NewWorkerPool(3, func(val int) (string, error) {
		return fmt.Sprintf("Got %d", val), nil
	})

	for range 10 {
		p.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	}
	p.Close()

	assert.False(t, p.IsErr())
	results, errors := p.Collect()
	assert.Nil(t, errors)
	assert.Equal(t, 100, len(results))
}

func TestWorkerErrors(t *testing.T) {
	p := NewWorkerPool(3, func(val int) (string, error) {
		return "", errors.New("oh no")
	})

	for range 10 {
		p.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	}
	p.Close()

	assert.True(t, p.IsErr())
	errors := p.TakeErrors()
	assert.Equal(t, 100, len(errors))
}

func TestWorkerOptions(t *testing.T) {
	p := NewWorkerPool(3, func(val int) (string, error) {
		return "", errors.New("oh no")
	}, WithBuffer(10), WithRetry(2))

	for range 10 {
		p.Push(0, 1, 2)
	}
	p.Close()

	assert.True(t, p.IsErr())
	errors := p.TakeErrors()
	assert.Equal(t, 60, len(errors))
}
