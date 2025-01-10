package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/arrno/gliter"
)

/*
In this example, we fetch paginated transaction records from an API, transform them
by two distinct transform functions, and store the results in a database.

By using a GLiter Pipeline, we can reduce ingest latency by storing records to the DB
while we're fetching pages from the API (and transforming). By running all stages
concurrently, the throughput is increased.

Since we have two transformers stacked in the middle stage, our pipeline is also forked
such that both downstream transform branches are writing to the DB concurrently.

All of this with almost zero async primitives.
*/

// These are our interfaces for input/output.

// API will be wrapped by our pipeline generator.
type API[T any] interface {
	FetchPage(page uint) (results []T, err error)
}

// DB is where our last pipeline stage will store the output.
type DB interface {
	BatchAdd(records []any) error
}

// Record is the pipeline input data type.
type Record struct {
	CreatedAt         time.Time
	TransactionNumber int
	AmountDollars     float64
}

// MockAPI satisfied the API interface.
type MockAPI[T any] struct {
	Pages [][]T
}

func NewMockAPI() *MockAPI[Record] {
	a := new(MockAPI[Record])
	a.Pages = make([][]Record, 10)
	for i := range 10 {
		page := make([]Record, 10)
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

// FetchPage returns a page of data that may be empty. If `page` is negative, an error is returned.
func (a *MockAPI[Record]) FetchPage(page int) (results []Record, err error) {
	if page < 1 {
		return nil, fmt.Errorf("page number must be positive. Received %d", page)
	} else if page > len(a.Pages) {
		return nil, nil
	}
	return a.Pages[page-1], nil
}

// MockDB satisfied the DB interface.
type MockDB struct {
	Records []any
}

func NewMockDB() *MockDB {
	return new(MockDB)
}

// BatchAdd stores the records. If `records` is empty, an error is returned.
func (db *MockDB) BatchAdd(records []any) error {
	if len(records) == 0 {
		return errors.New("records should not be empty")
	}
	db.Records = append(db.Records, records...)
	return nil
}

// RecordCents is a transform variant of the original `Record` type.
type RecordCents struct {
	CreatedAt         time.Time
	UpdatedAt         time.Time
	TransactionNumber int
	AmountCents       int
}

// ConvertToCents is our first transform function which converts a `Record` into a `RecordCents`.
func ConvertToCents(data any) (any, error) {
	records, ok := data.([]Record)
	if !ok {
		return data, errors.New("expected page in dollars")
	}
	results := make([]RecordCents, len(records))
	for i, r := range records {
		results[i] = RecordCents{
			CreatedAt:         r.CreatedAt,
			UpdatedAt:         time.Now(),
			TransactionNumber: r.TransactionNumber,
			AmountCents:       int(r.AmountDollars * 100),
		}
	}
	return results, nil
}

// ApplyEvenFee is our second transform function which applies a $10 fee to even-numbered Record transactions.
func ApplyEvenFee(data any) (any, error) {
	records, ok := data.([]Record)
	if !ok {
		return data, errors.New("expected page in dollars")
	}
	results := make([]Record, len(records))
	for i, r := range records {
		// Clone ptr to mutate
		results[i] = Record{
			CreatedAt:         r.CreatedAt,
			TransactionNumber: r.TransactionNumber,
			AmountDollars:     r.AmountDollars - 10.0,
		}
		if r.TransactionNumber%2 != 0 {
			results[i].AmountDollars -= 10
		}
	}
	return results, nil
}

// ExampleMain is the `Main` function for our example. Here we will build and run our pipeline.
func ExampleMain() {

	// First we create a generator that wraps our API service.
	gen := func() func() (any, bool, error) {
		api := NewMockAPI()
		var page int
		return func() (any, bool, error) {
			page++
			results, err := api.FetchPage(page)
			if err != nil {
				return struct{}{}, false, err
			}
			if len(results) == 0 {
				return results, false, nil
			} else {
				return results, true, nil
			}
		}
	}

	// Next we create a DB service and a `store` method.
	// The `store` method wraps the db `BatchAdd` function and will serve
	// as the final stage of our pipeline. Note that `store` also takes a
	// tally channel for granular processed-record counting.
	db := NewMockDB()
	store := func(inbound any, tally chan<- int) (any, error) {
		var results []any
		if records, ok := inbound.([]Record); ok {
			results = make([]any, len(records))
			for i, r := range records {
				results[i] = r
			}
		} else if records, ok := inbound.([]RecordCents); ok {
			results = make([]any, len(records))
			for i, r := range records {
				results[i] = r
			}
		}
		tally <- len(results) // count total records processed
		if err := db.BatchAdd(results); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}

	// `sf` is a type alias to make the code easier to read.
	type sf []func(any) (any, error)

	// We create a new pipeline out of our generator.
	pipeline := gliter.NewPipeline(gen())

	// Let's initialize a tally channel for better control over the processed count.
	tally := pipeline.Tally()
	// `storeWithTally` is a simple closure that captures `tally` and calls `store`.
	storeWithTally := func(inbound any) (any, error) {
		return store(inbound, tally)
	}

	// Lastly, we assemble our pipeline stages, enable logging, and run the pipeline.
	count, err := pipeline.
		Config(gliter.PLConfig{LogCount: true}).
		Stage(sf{
			ConvertToCents,
			ApplyEvenFee,
		}).
		Stage(sf{
			storeWithTally,
		}).
		Run()

	if err != nil {
		panic(err)
	}

	// We expect 200 records to have been processed by the sum of all `store` function instances.
	fmt.Printf("Node: %s\nCount: %d\n", count[0].NodeID, count[0].Count)

	// That's it!
}
