package gliter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	b := NewWorkerPool(3, func(val int) (string, error) {
		return fmt.Sprintf("Got %d", val), nil
	})

	for range 10 {
		err := b.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		if err != nil {
			panic(err)
		}
	}
	b.Close()

	assert.False(t, b.IsErr())
	results, errors := b.Collect()
	assert.Nil(t, errors)
	assert.Equal(t, 100, len(results))
}

func TestWorkerErrors(t *testing.T) {
	b := NewWorkerPool(3, func(val int) (string, error) {
		return "", errors.New("oh no")
	})

	for range 10 {
		err := b.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		if err != nil {
			panic(err)
		}
	}
	b.Close()

	assert.True(t, b.IsErr())
	errors := b.TakeErrors()
	assert.Equal(t, 100, len(errors))
}
