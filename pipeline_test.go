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
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleEnd,
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
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid,    // branch A
			exampleMidErr, // branch B
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.NotNil(t, err)

	// With throttle
	_, exampleEnd = makeEnd()
	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid,    // branch A
			exampleMidErr, // branch B
		).
		Stage(
			exampleEnd,
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
				exampleMid,
			).
			Stage(
				exampleEnd,
			).
			Run()
		assert.NotNil(t, err)
	}
}

func TestPipelineFork(t *testing.T) {
	col, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branches A.C, B.C
			exampleMid, // branches A.D, B.D
			exampleMid, // branches A.E, B.E
		).
		Stage(
			exampleEnd,
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
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branches A.C, B.C
			exampleMid, // branches A.D, B.D
			exampleMid, // branches A.E, B.E
		).
		Throttle(2). // merge into branches X, Z
		Stage(
			exampleEnd,
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

func TestPipelineTally(t *testing.T) {
	_, exampleEnd := makeEnd()

	pipeline := NewPipeline(exampleGen(5))
	tally := pipeline.Tally()

	endWithTally := func(i int) (int, error) {
		tally <- 1
		return exampleEnd(i)
	}

	count, err := pipeline.
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branches A.C, B.C
			exampleMid, // branches A.D, B.D
			exampleMid, // branches A.E, B.E
		).
		Stage(
			endWithTally,
		).
		Run()

	// 1, 2, 3, 4, 5
	// 16, 64, 144, 246, 400
	assert.Nil(t, err)
	assert.Equal(t, len(count), 1)
	assert.Equal(t, count[0].NodeID, "tally")
	assert.Equal(t, count[0].Count, 30)
}

func TestEmptyStage(t *testing.T) {
	count, err := NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Stage().
		Run()
	assert.Nil(t, err)
	assert.Equal(t, count[0].Count, 5)
	//
	count, err = NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Throttle(0).
		Run()
	assert.Nil(t, err)
	assert.Equal(t, count[0].Count, 5)
}

func TestPipelineBatch(t *testing.T) {
	col, exampleEnd := makeEndBatch()
	_, err := NewPipeline(exampleGen(23)).
		Batch(
			10,
			exampleEnd,
		).
		Run()
	assert.Nil(t, err)
	expected := make([]int, 23)
	for i := range 23 {
		expected[i] = (i + 1) * 2
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

func exampleGen(n int) func() (int, bool, error) {
	data := make([]int, n)
	for i := range n {
		data[i] = i + 1
	}
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

func makeEndBatch() (*Collect[int], func(set []int) ([]int, error)) {
	col := NewCollect[int]()
	return col, func(set []int) ([]int, error) {
		for _, j := range set {
			r := j * 2
			col.mu.Lock()
			col.items = append(col.items, r)
			col.mu.Unlock()
		}
		return nil, nil
	}
}

func exampleMidErr(i int) (int, error) {
	if i > 2 {
		return 0, errors.New("err")
	}
	return i * 2, nil
}
