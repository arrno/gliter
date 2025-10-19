package main

import (
	"fmt"

	"github.com/arrno/gliter"
)

// Example wires together the ecommerce demo pipeline. Supporting mocks and handlers live in
// demo_support.go so the pipeline flow stays easy to read.
func Example() {
	// Make a pipeline that
	// - Streams API pages
	// - Fans out/in to do computations on multiple pages at a time
	// - Forks out each page for concurrent enrichment and merges results
	// - Stores each page to db and search index concurrently
	// - Retries on failure, auto backpressure

	resetDemoState()

	counts, err := gliter.NewPipeline(
		streamFetch,
		gliter.WithReturnCount(),
	).
		WorkPool(
			processData,
			cfg.processWorkers,
			gliter.WithRetry(cfg.processRetry),
			gliter.WithBuffer(cfg.processBuffer),
		).
		ForkOutIn([]func(data any) (any, error){
			enrichRate,
			enrichScore,
			enrichRelated,
		},
			mergeEnriched,
			gliter.WithRetry(cfg.enrichmentRetry),
			gliter.WithBuffer(cfg.enrichmentBuffer),
		).
		ForkOutIn([]func(data any) (any, error){
			storeDB,
			indexSearch,
		},
			nil, // No merge
			gliter.WithBuffer(cfg.storageBuffer),
			gliter.WithRetry(cfg.storageRetry),
		).
		Run()

	if err != nil {
		fmt.Printf("pipeline error: %v\n", err)
		return
	}

	if err := validateDemo(counts); err != nil {
		fmt.Println(err)
		return
	}

	storeMu.Lock()
	pagesProcessed := len(storedPages)
	storeMu.Unlock()
	indexMu.Lock()
	indexedDocs := len(indexDocs)
	indexMu.Unlock()

	fmt.Printf("demo OK: processed %d pages, indexed %d items\n", pagesProcessed, indexedDocs)
}

func main() {
	Example()
}
