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
	_, err := NewPipeline(exampleGen()).
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
	_, err := NewPipeline(exampleGen()).
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
	_, err = NewPipeline(exampleGen()).
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

func TestPipelineGenErr(t *testing.T) {
	for _, gen := range []func() (int, bool, error){
		exampleGenErrOne(),
		exampleGenErrTwo(),
		exampleGenErrThree(),
	} {
		_, exampleEnd := makeEnd()
		_, err := NewPipeline(gen).
			Stage(
				[]func(i int) (int, error){
					exampleMid,
				},
			).
			Stage(
				[]func(i int) (int, error){
					exampleEnd,
				},
			).
			Run()
		assert.NotNil(t, err)
	}
}

func TestPipelineFork(t *testing.T) {
	col, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen()).
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
	_, err := NewPipeline(exampleGen()).
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

func exampleGen() func() (int, bool, error) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool, error) {
		index++
		if index == len(data) {
			return 0, false, nil
		}
		return data[index], true, nil
	}
}

func exampleGenErrOne() func() (int, bool, error) {
	return func() (int, bool, error) {
		return 0, true, errors.New("err")
	}
}

func exampleGenErrTwo() func() (int, bool, error) {
	return func() (int, bool, error) {
		return 0, false, errors.New("err")
	}
}

func exampleGenErrThree() func() (int, bool, error) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool, error) {
		index++
		if index > 1 {
			return 0, false, errors.New("err")
		}
		if index == len(data) {
			return 0, false, nil
		}
		return data[index], true, nil
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

func TestPipelineTally(t *testing.T) {
	_, exampleEnd := makeEnd()

	pipeline := NewPipeline(exampleGen())
	tally := pipeline.Tally()

	endWithTally := func(i int) (int, error) {
		tally <- 1
		return exampleEnd(i)
	}

	count, err := pipeline.
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
		Stage(
			[]func(i int) (int, error){
				endWithTally,
			},
		).
		Run()

	// 1, 2, 3, 4, 5
	// 16, 64, 144, 246, 400
	assert.Nil(t, err)
	assert.Equal(t, len(count), 1)
	assert.Equal(t, count[0].NodeID, "tally")
	assert.Equal(t, count[0].Count, 30)
}
