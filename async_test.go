package gliter

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInParallel(t *testing.T) {
	tasks := []func() (string, error){
		func() (string, error) {
			return "Hello", nil
		},
		func() (string, error) {
			return ", ", nil
		},
		func() (string, error) {
			return "Async!", nil
		},
	}
	results, err := InParallel(tasks)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual([]string{"Hello", ", ", "Async!"}, results))
}

func TestInParallelErr(t *testing.T) {
	tasks := []func() (string, error){
		func() (string, error) {
			return "Hello", nil
		},
		func() (string, error) {
			return ",", errors.New("error")
		},
		func() (string, error) {
			return "Async!", nil
		},
	}
	_, err := InParallel(tasks)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "error")
}

func TestIterDoneLong(t *testing.T) {
	done := make(chan any)
	data := make(chan int)
	go func() {
		defer close(data)
		for i := range 100 {
			data <- i
		}
	}()

	results := make([]int, 100)
	expected := make([]int, 100)

	for i := range IterDone(data, done) {
		results[i] = i
	}
	for i := range 100 {
		expected[i] = i
	}
	assert.True(t, reflect.DeepEqual(expected, results))
}

func TestIterDoneShort(t *testing.T) {
	done := make(chan any)
	final := make(chan any)
	data := make(chan int)
	go func() {
		defer close(data)
		for i := range 21 {
			data <- i
		}
		close(done) // simulate disrupt
		<-final     // hang so data is not closed
	}()

	results := make([]int, 100)
	expected := make([]int, 100)

	for i := range IterDone(data, done) {
		results[i] = i
	} // can only exit due to done being closed
	for i := range 21 {
		expected[i] = i
	}
	assert.True(t, reflect.DeepEqual(expected, results))
	// close final so goroutine can dismount and close data
	close(final)
}
