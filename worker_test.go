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

	err := b.WithBuffSize(30)
	if err != nil {
		panic(err)
	}

	b.Boot()
	for range 10 {
		err := b.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		if err != nil {
			panic(err)
		}
	}
	err = b.Close()
	if err != nil {
		panic(err)
	}

	assert.False(t, b.IsErr())
	results := b.TakeResults()
	assert.Equal(t, 100, len(results))
}

func TestWorkerErrors(t *testing.T) {
	b := NewWorkerPool(3, func(val int) (string, error) {
		return "", errors.New("oh no")
	})

	err := b.WithBuffSize(30)
	if err != nil {
		panic(err)
	}

	b.Boot()
	for range 10 {
		err := b.Push(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		if err != nil {
			panic(err)
		}
	}
	err = b.Close()
	if err != nil {
		panic(err)
	}

	assert.True(t, b.IsErr())
	errors := b.TakeErrors()
	assert.Equal(t, 100, len(errors))
}
