package gliter

import (
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	col, exampleEnd := makeEnd()
	err := NewPipeline(exampleGen()).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branch A
				exampleMid, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Run()
	assert.Nil(t, err)
	expected := []int{4, 4, 16, 16, 36, 36, 64, 64, 100, 100}
	actual := col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
}

func TestPipelineErr(t *testing.T) {
	_, exampleEnd := makeEnd()
	err := NewPipeline(exampleGen()).
		Stage(
			[]func(i int) (int, error){
				exampleMid,    // branch A
				exampleMidErr, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Run()
	assert.NotNil(t, err)

	// With throttle
	_, exampleEnd = makeEnd()
	err = NewPipeline(exampleGen()).
		Stage(
			[]func(i int) (int, error){
				exampleMid,    // branch A
				exampleMidErr, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Throttle(1).
		Run()
	assert.NotNil(t, err)
}

func TestPipelineFork(t *testing.T) {
	col, exampleEnd := makeEnd()
	err := NewPipeline(exampleGen()).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branch A
				exampleMid, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branches A.C B.C
				exampleMid, // branches A.D B.D
				exampleMid, // branches A.E B.E
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Run()
	// 1, 2, 3, 4, 5
	// 16, 64, 144, 246, 400
	assert.Nil(t, err)
	expected := []int{
		16, 16, 16, 16, 16, 16,
		64, 64, 64, 64, 64, 64,
		144, 144, 144, 144, 144, 144,
		256, 256, 256, 256, 256, 256,
		400, 400, 400, 400, 400, 400,
	}
	actual := col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
}

func TestPipelineThrottle(t *testing.T) {
	col, exampleEnd := makeEnd()
	err := NewPipeline(exampleGen()).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branch A
				exampleMid, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branches A.C, B.C
				exampleMid, // branches A.D, B.D
				exampleMid, // branches A.E, B.E
			},
		).
		Throttle(2). // merge into branches X, Z
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Run()
	// 1, 2, 3, 4, 5
	// 16, 64, 144, 246, 400
	assert.Nil(t, err)
	expected := []int{
		16, 16, 16, 16, 16, 16,
		64, 64, 64, 64, 64, 64,
		144, 144, 144, 144, 144, 144,
		256, 256, 256, 256, 256, 256,
		400, 400, 400, 400, 400, 400,
	}
	actual := col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
}

type Collect[T any] struct {
	mu    sync.Mutex
	items []T
}

func NewCollect[T any]() *Collect[T] {
	return &Collect[T]{
		items: []T{},
	}
}

func exampleGen() func() (int, bool) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool) {
		index++
		if index == len(data) {
			return 0, false
		}
		return data[index], true
	}
}

func exampleMid(i int) (int, error) {
	return i * 2, nil
}

func makeEnd() (*Collect[int], func(i int) (int, error)) {
	col := NewCollect[int]()
	return col, func(i int) (int, error) {
		r := i * i
		col.mu.Lock()
		defer col.mu.Unlock()
		col.items = append(col.items, r)
		return r, nil
	}
}

func exampleMidErr(i int) (int, error) {
	if i > 2 {
		return 0, errors.New("err")
	}
	return i * 2, nil
}
