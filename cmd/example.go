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

// Transform types
type RecordCents struct {
	CreatedAt         time.Time
	UpdatedAt         time.Time
	TransactionNumber int
	AmountCents       int
}

// Union type
type Union struct {
	PageDollars []Record
	PageCents   []RecordCents
}

func ToUnion(p []Record) Union {
	return Union{
		PageDollars: p,
	}
}

// Transform 1
func ConvertToCents(du Union) (Union, error) {
	p := du.PageDollars
	if p == nil {
		return du, errors.New("expected page in dollars")
	}
	cu := Union{
		PageCents: make([]RecordCents, len(p)),
	}
	for i := range cu.PageCents {
		cu.PageCents[i] = RecordCents{
			CreatedAt:         p[i].CreatedAt,
			UpdatedAt:         time.Now(),
			TransactionNumber: p[i].TransactionNumber,
			AmountCents:       int(p[i].AmountDollars * 100),
		}
	}
	return cu, nil
}

// Transform 2
func ApplyEvenFee(du Union) (Union, error) {
	p := du.PageDollars
	if p == nil {
		return du, errors.New("expected page in dollars")
	}
	nu := Union{
		PageDollars: make([]Record, len(p)),
	}
	for i := range nu.PageDollars {
		// Clone ptr to mutate
		nu.PageDollars[i] = Record{
			CreatedAt:         p[i].CreatedAt,
			TransactionNumber: p[i].TransactionNumber,
			AmountDollars:     p[i].AmountDollars - 10.0,
		}
		if p[i].TransactionNumber%2 != 0 {
			nu.PageDollars[i].AmountDollars -= 10
		}
	}
	return nu, nil
}

func Stream() {

	gen := func() func() (Union, bool) {
		api := NewMockAPI()
		var page int
		return func() (Union, bool) {
			page++
			results, err := api.FetchPage(page)
			if err != nil {
				panic(err)
			}
			if len(results) == 0 {
				return ToUnion(results), false
			} else {
				return ToUnion(results), true
			}
		}
	}

	db := NewMockDB()

	store := func(inbound Union) (Union, error) {
		var records []any
		if inbound.PageCents != nil {
			records = make([]any, len(inbound.PageCents))
			for i, r := range inbound.PageCents {
				records[i] = r
			}
		} else if inbound.PageDollars != nil {
			records = make([]any, len(inbound.PageDollars))
			for i, r := range inbound.PageDollars {
				records[i] = r
			}
		}
		if err := db.BatchAdd(records); err != nil {
			return Union{}, err
		}
		return Union{}, nil
	}

	// Stage functions alias
	type sf []func(Union) (Union, error)

	gliter.NewPipeline(gen()).
		Stage(sf{
			ConvertToCents,
			ApplyEvenFee,
		}).
		Stage(sf{
			store,
		}).
		Run()
}
