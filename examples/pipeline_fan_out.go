package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/arrno/gliter"
)

// To simplify the example, we use a generic API that fetches pages of []any

func NewMockAPIAny() *MockAPI[any] {
	a := new(MockAPI[any])
	a.Pages = make([][]any, 10)
	for i := range 10 {
		page := make([]any, 10)
		for j := range len(page) {
			page[j] = Record{
				CreatedAt:         time.Now(),
				TransactionNumber: (j * i) + i,
				AmountDollars:     math.Round(rand.Float64()*10000) / 100,
			}
		}
		a.Pages[i] = page
	}
	return a
}

func (a *MockAPI[any]) FetchPageAny(page int) (results []any, err error) {
	if page < 1 {
		return nil, fmt.Errorf("page number must be positive. Received %d", page)
	} else if page > len(a.Pages) {
		return nil, nil
	}
	return a.Pages[page-1], nil
}

// ExampleFanOut is the `Main` function for our example. Here we will build and run our pipeline.
func ExampleFanOut() {

	// First we create a generator that wraps our API service.
	gen := func() func() ([]any, bool, error) {
		api := NewMockAPIAny()
		var page int
		return func() ([]any, bool, error) {
			page++
			results, err := api.FetchPage(page)
			if err != nil {
				return nil, false, err
			}
			if len(results) == 0 {
				return results, false, nil
			} else {
				return results, true, nil
			}
		}
	}

	// Next, we create a simple time stamping function for our middle stage
	wrap := func(inbound []any) ([]any, error) {
		wrappedData := make([]any, len(inbound))
		for i, data := range inbound {
			wrappedData[i] = map[string]any{
				"processedAt": time.Now(),
				"data":        data,
			}
		}
		return wrappedData, nil
	}

	// init mock db
	db := NewMockDB()

	// Finally, the interesting part. In our store function, we chunk the records
	// and write all chunks in parallel.
	fanOutStore := func(inbound []any) ([]any, error) {
		chunks := gliter.ChunkBy(inbound, 5)
		funcs := make([]func() (any, error), len(chunks))
		for i, chunk := range chunks {
			funcs[i] = func() (any, error) {
				// We only care about the error
				return nil, db.BatchAdd(chunk)
			}
		}
		_, err := gliter.InParallel(funcs)
		return nil, err
	}

	// assemble and run the pipeline
	_, err := gliter.NewPipeline(
		gen(),
		gliter.WithLogCount(),
	).
		Stage(wrap).
		Stage(fanOutStore).
		Run()

	if err != nil {
		panic(err)
	}

	// Unlike in pipeline_example.go, this pipeline only processes 10pages * 10records = 100 records (no forking here).
	// Even so, the final stage called 1:0:0 is writing two concurrent chunks of 5records each on every invocation...
	// so while GEN, 0:0:0, and 1:0:0 each have only 10 invocations, 1:0:0 has spawned a total of 20 child go routines.
	// +-------+-------+----------------------+
	// | node  | count | value                |
	// +-------+-------+----------------------+
	// | GEN   | 10    | [{2025-02-03 12:45.. |
	// | 0:0:0 | 10    | [map[data:{2025-02.. |
	// | 1:0:0 | 10    | []                   |
	// +-------+-------+----------------------+

	// That's it!
}
