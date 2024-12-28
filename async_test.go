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
