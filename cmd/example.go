package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/arrno/gliter"
)

// Interfaces for input/output.
type API[T any] interface {
	FetchPage(page uint) (results []T, err error)
}

type DB interface {
	BatchAdd(records []any) error
}

// Original data type
type Record struct {
	CreatedAt         time.Time
	TransactionNumber int
	AmountDollars     float64
}

// API
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

func (a *MockAPI[Record]) FetchPage(page int) (results []Record, err error) {
	if page < 1 {
		return nil, fmt.Errorf("page number must be positive. Received %d", page)
	} else if page > len(a.Pages) {
		return nil, nil
	}
	return a.Pages[page-1], nil
}

// DB
type MockDB struct {
	Records []any
}

func NewMockDB() *MockDB {
	return new(MockDB)
}

func (db *MockDB) BatchAdd(records []any) error {
	if len(records) == 0 {
		return errors.New("records should not be empty")
	}
	db.Records = append(db.Records, records...)
	return nil
}

// Transform type
type RecordCents struct {
	CreatedAt         time.Time
	UpdatedAt         time.Time
	TransactionNumber int
	AmountCents       int
}

// Transform 1
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

// Transform 2
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

// Example main function to stream the data in a pipeline
func ExampleMain() {

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

	db := NewMockDB()

	store := func(inbound any) (any, error) {
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
		if err := db.BatchAdd(results); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}

	// Stage functions alias
	type sf []func(any) (any, error)

	_, err := gliter.NewPipeline(gen()).
		Config(gliter.PLConfig{LogCount: true}).
		Stage(sf{
			ConvertToCents,
			ApplyEvenFee,
		}).
		Stage(sf{
			store,
		}).
		Run()

	if err != nil {
		panic(err)
	}
}
