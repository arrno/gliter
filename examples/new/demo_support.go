package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arrno/gliter"
)

type Product struct {
	ID             string
	Name           string
	Category       string
	Price          float64
	Inventory      int
	Rating         float64
	Score          float64
	ConversionRate float64
	Related        []string
}

type PageSummary struct {
	ItemCount      int
	AveragePrice   float64
	TotalInventory int
	DroppedSKUs    []string
}

type ProductPage struct {
	Page      int
	Cursor    string
	Items     []Product
	Retrieved time.Time
	Processed time.Time
	Summary   PageSummary
	Validated bool
}

type SearchDocument struct {
	SKU       string
	Name      string
	Category  string
	Score     float64
	Rate      float64
	InStock   bool
	Boostable bool
}

var (
	streamIdx int

	cfg = struct {
		fetchDelay       time.Duration
		processWorkers   uint
		processBuffer    int
		processRetry     int
		enrichmentBuffer int
		enrichmentRetry  int
		storageBuffer    int
		storageRetry     int
	}{
		fetchDelay:       40 * time.Millisecond,
		processWorkers:   4,
		processBuffer:    8,
		processRetry:     3,
		enrichmentBuffer: 5,
		enrichmentRetry:  2,
		storageBuffer:    4,
		storageRetry:     2,
	}

	demoPages = []ProductPage{
		{
			Page:   1,
			Cursor: "cursor-a",
			Items: []Product{
				{ID: "SKU-100", Name: "Nova Sneakers", Category: "footwear", Price: 129.99, Inventory: 12, Rating: 4.6},
				{ID: "SKU-101", Name: "Nebula Jacket", Category: "outerwear", Price: 249.50, Inventory: 3, Rating: 4.9},
				{ID: "SKU-102", Name: "Comet Socks", Category: "accessories", Price: 19.95, Inventory: 0, Rating: 4.1},
			},
		},
		{
			Page:   2,
			Cursor: "cursor-b",
			Items: []Product{
				{ID: "SKU-200", Name: "Aurora Tee", Category: "tops", Price: 38.00, Inventory: 34, Rating: 4.2},
				{ID: "SKU-201", Name: "Orbit Belt", Category: "accessories", Price: 54.00, Inventory: 18, Rating: 4.4},
				{ID: "SKU-202", Name: "Halo Ring", Category: "jewelry", Price: 320.00, Inventory: 1, Rating: 4.8},
			},
		},
		{
			Page:   3,
			Cursor: "cursor-c",
			Items: []Product{
				{ID: "SKU-300", Name: "Galaxy Hoodie", Category: "outerwear", Price: 158.00, Inventory: 9, Rating: 4.3},
				{ID: "SKU-301", Name: "Quasar Hat", Category: "headwear", Price: 42.00, Inventory: 0, Rating: 3.9},
				{ID: "SKU-302", Name: "Meteor Gloves", Category: "accessories", Price: 65.00, Inventory: 16, Rating: 4.5},
			},
		},
	}

	relatedCatalog = map[string][]string{
		"SKU-100": {"SKU-200", "SKU-302"},
		"SKU-101": {"SKU-300"},
		"SKU-200": {"SKU-100", "SKU-201"},
		"SKU-202": {"SKU-302"},
		"SKU-300": {"SKU-101", "SKU-100"},
	}

	storeMu     sync.Mutex
	storedPages []*ProductPage

	indexMu   sync.Mutex
	indexDocs []SearchDocument
)

func clonePage(src *ProductPage) *ProductPage {
	if src == nil {
		return nil
	}
	copy := *src
	copy.Items = make([]Product, len(src.Items))
	copy.Summary.DroppedSKUs = append([]string(nil), src.Summary.DroppedSKUs...)
	copy.Validated = src.Validated
	for i, item := range src.Items {
		copy.Items[i] = item
	}
	return &copy
}

func resetDemoState() {
	streamIdx = 0
	storeMu.Lock()
	storedPages = storedPages[:0]
	storeMu.Unlock()
	indexMu.Lock()
	indexDocs = indexDocs[:0]
	indexMu.Unlock()
}

func validateDemo(counts []gliter.PLNodeCount) error {
	storeMu.Lock()
	stored := make([]*ProductPage, len(storedPages))
	copy(stored, storedPages)
	storeMu.Unlock()

	if len(stored) != len(demoPages) {
		return fmt.Errorf("validation failed: stored %d pages, expected %d", len(stored), len(demoPages))
	}

	indexMu.Lock()
	docs := make([]SearchDocument, len(indexDocs))
	copy(docs, indexDocs)
	indexMu.Unlock()

	expectedDocs := 0
	for _, page := range stored {
		if page.Summary.ItemCount != len(page.Items) {
			return fmt.Errorf("validation failed: page %d summary mismatch", page.Page)
		}
		for _, item := range page.Items {
			if item.Score <= 0 {
				return fmt.Errorf("validation failed: item %s missing score", item.ID)
			}
			if item.ConversionRate <= 0 {
				return fmt.Errorf("validation failed: item %s missing conversion", item.ID)
			}
		}
		expectedDocs += len(page.Items)
		page.Validated = true
	}

	if len(docs) != expectedDocs {
		return fmt.Errorf("validation failed: indexed %d docs, expected %d", len(docs), expectedDocs)
	}

	genCount := -1
	for _, count := range counts {
		if count.NodeID == "GEN" {
			genCount = count.Count
			break
		}
	}
	if genCount != -1 && genCount != len(demoPages) {
		return fmt.Errorf("validation failed: generator count %d, expected %d", genCount, len(demoPages))
	}

	return nil
}

func asPage(val any, op string) (*ProductPage, error) {
	page, ok := val.(*ProductPage)
	if !ok {
		return nil, fmt.Errorf("%s: expected *ProductPage got %T", op, val)
	}
	return page, nil
}

func streamFetch() (any, bool, error) {
	if streamIdx >= len(demoPages) {
		return nil, false, nil
	}
	page := clonePage(&demoPages[streamIdx])
	page.Retrieved = time.Now()
	streamIdx++

	if cfg.fetchDelay > 0 {
		time.Sleep(cfg.fetchDelay)
	}

	return page, true, nil
}

func processData(val any) (any, error) {
	page, err := asPage(val, "processData")
	if err != nil {
		return nil, err
	}

	filtered := make([]Product, 0, len(page.Items))
	dropped := make([]string, 0, len(page.Items))
	var totalPrice float64
	var totalInventory int

	for _, item := range page.Items {
		if item.Inventory <= 0 {
			dropped = append(dropped, item.ID)
			continue
		}
		filtered = append(filtered, item)
		totalPrice += item.Price
		totalInventory += item.Inventory
	}

	page.Items = filtered
	page.Processed = time.Now()
	page.Summary.ItemCount = len(filtered)
	if len(filtered) > 0 {
		page.Summary.AveragePrice = math.Round((totalPrice/float64(len(filtered)))*100) / 100
	}
	page.Summary.TotalInventory = totalInventory
	page.Summary.DroppedSKUs = dropped

	return page, nil
}

func enrichScore(val any) (any, error) {
	page, err := asPage(val, "enrichScore")
	if err != nil {
		return nil, err
	}

	clone := clonePage(page)
	for i := range clone.Items {
		item := &clone.Items[i]
		score := item.Rating * 18
		if item.Price > 200 {
			score -= 6
		}
		if item.Inventory < 3 {
			score -= 4
		} else if item.Inventory > 20 {
			score += 3
		}
		score = math.Max(10, math.Min(score, 100))
		item.Score = math.Round(score*100) / 100
	}

	return clone, nil
}

func enrichRate(val any) (any, error) {
	page, err := asPage(val, "enrichRate")
	if err != nil {
		return nil, err
	}

	clone := clonePage(page)
	for i := range clone.Items {
		item := &clone.Items[i]
		base := 0.05 + (item.Rating-3.5)*0.015
		if item.Price > 150 {
			base -= 0.01
		}
		if item.Inventory < 5 {
			base -= 0.005
		}
		if item.Inventory > 20 {
			base += 0.007
		}
		base = math.Max(0.01, math.Min(base, 0.45))
		item.ConversionRate = math.Round(base*1000) / 1000
	}

	return clone, nil
}

func enrichRelated(val any) (any, error) {
	page, err := asPage(val, "enrichRelated")
	if err != nil {
		return nil, err
	}

	clone := clonePage(page)
	for i := range clone.Items {
		item := &clone.Items[i]
		if related, ok := relatedCatalog[item.ID]; ok {
			item.Related = append([]string(nil), related...)
		}
	}

	return clone, nil
}

func mergeEnriched(val []any) ([]any, error) {
	if len(val) == 0 {
		return nil, nil
	}

	var merged *ProductPage
	index := make(map[string]*Product)

	for _, item := range val {
		page, err := asPage(item, "mergeEnriched")
		if err != nil {
			return nil, err
		}

		if merged == nil {
			merged = clonePage(page)
			index = make(map[string]*Product, len(merged.Items))
			for i := range merged.Items {
				index[merged.Items[i].ID] = &merged.Items[i]
			}
			continue
		}

		for i := range page.Items {
			incoming := page.Items[i]
			target, ok := index[incoming.ID]
			if !ok {
				merged.Items = append(merged.Items, incoming)
				index[incoming.ID] = &merged.Items[len(merged.Items)-1]
				continue
			}
			if incoming.Score > 0 {
				target.Score = incoming.Score
			}
			if incoming.ConversionRate > 0 {
				target.ConversionRate = incoming.ConversionRate
			}
			if len(incoming.Related) > 0 {
				target.Related = append([]string(nil), incoming.Related...)
			}
		}
	}

	return []any{merged}, nil
}

func storeDB(val any) (any, error) {
	page, err := asPage(val, "storeDB")
	if err != nil {
		return nil, err
	}

	storeMu.Lock()
	storedPages = append(storedPages, clonePage(page))
	storeMu.Unlock()

	return page, nil
}

func indexSearch(val any) (any, error) {
	page, err := asPage(val, "indexSearch")
	if err != nil {
		return nil, err
	}

	docs := make([]SearchDocument, 0, len(page.Items))
	for _, item := range page.Items {
		doc := SearchDocument{
			SKU:       item.ID,
			Name:      item.Name,
			Category:  item.Category,
			Score:     item.Score,
			Rate:      item.ConversionRate,
			InStock:   item.Inventory > 0,
			Boostable: item.Score > 70 && item.ConversionRate >= 0.08,
		}
		docs = append(docs, doc)
	}

	indexMu.Lock()
	indexDocs = append(indexDocs, docs...)
	indexMu.Unlock()

	return page, nil
}
