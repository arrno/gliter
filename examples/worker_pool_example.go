package main

import (
	"fmt"
	"time"

	"github.com/arrno/gliter"
)

func simple() {
	handler := func(val int) (string, error) {
		return fmt.Sprintf("Got %d", val), nil
	}

	results, errors := gliter.NewWorkerPool(3, handler).
		Push(1, 2, 3, 4, 5).
		Close().
		Collect()

	if errors != nil {
		panic(errors)
	}
	fmt.Println(results)
}

func periodic() {
	// - START -
	handler := func(val int) (string, error) {
		return fmt.Sprintf("Got %d", val), nil
	}

	b := gliter.NewWorkerPool(3, handler) // make and spawn

	// - RUN -
	for range 10 {
		b.Push(0, 1, 2, 3)
		time.Sleep(100 * time.Millisecond)
		// periodically check/drain worker state
		results, errors := b.Collect()
		if errors != nil {
			panic(errors)
		}
		fmt.Println(results)
	}

	// - CLOSE -
	results, errors := b.Close().Collect() // waits for remaining work to complete
	if errors != nil {
		panic(errors)
	}
	fmt.Println(results)

}
