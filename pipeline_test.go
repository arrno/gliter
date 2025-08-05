package gliter

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
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

func TestPipelineEndErr(t *testing.T) {
	_, endErr := makeEndErr()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid,
		).
		Stage(
			endErr,
		).
		Run()
	assert.NotNil(t, err)

	_, endBatchErr := makeEndBatchErr()
	_, err = NewPipeline(exampleGen(5)).
		Batch(
			10,
			endBatchErr,
		).
		Run()
	assert.NotNil(t, err)
}

func TestPipelineTallyErr(t *testing.T) {
	_, exampleEnd := makeEnd()

	pipeline := NewPipeline(exampleGenErrFour())
	tally := pipeline.Tally()

	endWithTally := func(i int) (int, error) {
		tally <- 1
		return exampleEnd(i)
	}

	_, err := pipeline.
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branches A.C, B.C
			exampleMid, // branches A.D, B.D
		).
		Stage(
			endWithTally,
			endWithTally,
			endWithTally,
		).
		Run()

	assert.NotNil(t, err)
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

func TestPipelineThrottleErr(t *testing.T) {
	_, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Throttle(3).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Equal(t, err, ErrInvalidThrottle)

	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Throttle(4).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Nil(t, err)

	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Throttle(5).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Equal(t, err, ErrInvalidThrottle)
}

func TestPipelineMerge(t *testing.T) {
	col, exampleEnd := makeNoopEnd()
	counts, err := NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Stage(
			exampleMid, // branches A.C, B.C
			exampleMid, // branches A.D, B.D
			exampleMid, // branches A.E, B.E
		).
		Merge(
			func(data []int) ([]int, error) {
				str := ""
				for _, n := range data {
					str += string([]rune(fmt.Sprintf("%d", n))[0])
				}
				i, err := strconv.ParseInt(str, 10, 64)
				if err != nil {
					return nil, err
				}
				return []int{int(i)}, nil
			},
		).
		Stage(
			exampleEnd,
		).
		Run()
	// 1, 2, 3, 4, 5
	// 4, 8, 12, 16, 20
	assert.Nil(t, err)
	expected := []int{
		111111,
		111111,
		222222,
		444444,
		888888,
	}
	actual := col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
	assert.Equal(t, 5, counts[len(counts)-1].Count)

	// With no forking
	col, exampleEnd = makeNoopEnd()
	counts, err = NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Stage(
			exampleMid, // branch A
		).
		Merge(
			func(items []int) ([]int, error) {
				sum := 0
				for _, item := range items {
					sum += item
				}
				return []int{sum}, nil
			},
		).
		Stage(
			exampleEnd,
		).
		Run()

	assert.Nil(t, err)
	expected = []int{2, 4, 6, 8, 10}

	actual = col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
	assert.Equal(t, 5, counts[len(counts)-1].Count)
}

func TestPipelineMergeErr(t *testing.T) {
	_, exampleEnd := makeNoopEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Merge(
			func(data []int) ([]int, error) {
				return nil, errors.New("oh no")
			},
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.NotNil(t, err)
}

func TestPipelineOption(t *testing.T) {
	_, exampleEnd := makeNoopEnd()
	counts, err := NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Option(
			func(item int) (int, error) {
				return 1, nil
			},
			func(item int) (int, error) {
				return 2, nil
			},
			func(item int) (int, error) {
				return 3, nil
			},
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Nil(t, err)
	expectedNodeIds := []string{
		"GEN",
		"0:0:0",
		"0:0:1",
		"1:0:0",
		"1:1:0",
		"2:0:0",
		"2:1:0",
	}
	expectedNodeCounts := []int{5, 5, 5, 5, 5, 5, 5}
	resultNodeIds := make([]string, 7)
	resultNodeCounts := make([]int, 7)
	for i, count := range counts {
		resultNodeIds[i] = count.NodeID
		resultNodeCounts[i] = count.Count
	}
	assert.Equal(t, expectedNodeIds, resultNodeIds)
	assert.Equal(t, expectedNodeCounts, resultNodeCounts)
}

func TestPipelineOptionErr(t *testing.T) {
	_, exampleEnd := makeNoopEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Option(
			func(data int) (int, error) {
				return 0, errors.New("oh no")
			},
			func(data int) (int, error) {
				return 0, errors.New("oh no")
			},
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.NotNil(t, err)
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

	// Empty generator
	_, err = NewPipeline[int](nil).
		Config(PLConfig{ReturnCount: true}).
		Stage(exampleMid).
		Run()
	assert.NotNil(t, err)
}

func TestNilStage(t *testing.T) {
	_, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			nil,
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrNilFunc))

	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid,
		).
		Batch(
			2, nil,
		).
		Run()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrNilFunc))

	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid,
		).
		Merge(nil).
		Run()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrNilFunc))

	_, err = NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid,
		).
		Option(
			exampleMid,
			nil,
		).
		Run()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrNilFunc))
}

func TestPipelineBatch(t *testing.T) {
	col, exampleEndBatch := makeEndBatch()
	_, err := NewPipeline(exampleGen(23)).
		Batch(
			10,
			exampleEndBatch,
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

	col, exampleEnd := makeEnd()
	_, err = NewPipeline(exampleGen(23)).
		Batch(
			10,
			exampleMidBatch,
		).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Nil(t, err)
	actual = col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	expected = make([]int, 23)
	for i := range 23 {
		v := (i + 1) * 2
		expected[i] = v * v
	}
	assert.True(t, reflect.DeepEqual(expected, actual))
}

func TestPipelineBatchBranch(t *testing.T) {
	col, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Batch(2, exampleMidBatch).
		Stage(
			exampleEnd,
		).
		Run()
	assert.Nil(t, err)
	expected := []int{16, 16, 64, 64, 144, 144, 256, 256, 400, 400}
	actual := col.items
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})
	assert.True(t, reflect.DeepEqual(expected, actual))
}

func TestPipelineBuffer(t *testing.T) {
	col, exampleEnd := makeEnd()
	_, err := NewPipeline(exampleGen(5)).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Buffer(5).
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

func TestPipelineMix(t *testing.T) {
	_, exampleEnd := makeEnd()
	count, err := NewPipeline(exampleGen(5)).
		Config(PLConfig{ReturnCount: true}).
		Stage(
			exampleMid, // branch A
			exampleMid, // branch B
		).
		Throttle(1). // merge into 1 branch
		Batch(5, exampleMidBatch).
		Buffer(2).
		Stage(
			exampleEnd,
		).
		Run()
	expected := []PLNodeCount{
		{
			NodeID: "GEN",
			Count:  5, // gen 5
		},
		{
			NodeID: "0:0:0",
			Count:  5, // branch a
		},
		{
			NodeID: "0:0:1",
			Count:  5, // branch b
		},
		// throttle 1
		{
			NodeID: "[THROTTLE]",
			Count:  -1, // throttle doesn't keep count
		},
		{
			NodeID: "2:0:0",
			Count:  2, // 5 per branch is 10 batched by 5 is 2
		},
		// buffer
		{
			NodeID: "[BUFFER]",
			Count:  10,
		},
		{
			NodeID: "4:0:0",
			Count:  10, // 10 total items un-batched
		},
	}
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(expected, count))
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

func exampleGenErrFour() func() (int, bool, error) {
	data := make([]int, 100)
	for i := range 100 {
		data[i] = i
	}
	index := -1
	return func() (int, bool, error) {
		index++
		if index > 75 {
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

func exampleMidBatch(s []int) ([]int, error) {
	results := make([]int, 0, len(s))
	for _, i := range s {
		results = append(results, i*2)
	}
	return results, nil
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

func makeNoopEnd() (*Collect[int], func(i int) (int, error)) {
	col := NewCollect[int]()
	return col, func(i int) (int, error) {
		col.mu.Lock()
		defer col.mu.Unlock()
		col.items = append(col.items, i)
		return i, nil
	}
}

func makeEndErr() (*Collect[int], func(i int) (int, error)) {
	col := NewCollect[int]()
	return col, func(i int) (int, error) {
		if i > 2 {
			return 0, errors.New("err")
		}
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

func makeEndBatchErr() (*Collect[int], func(set []int) ([]int, error)) {
	col := NewCollect[int]()
	return col, func(set []int) ([]int, error) {
		for _, j := range set {
			if j > 2 {
				return nil, errors.New("err")
			}
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
