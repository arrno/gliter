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

	results, errors := gliter.NewWorkerPool(3, handler, gliter.WithRetry(2)).
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

	p := gliter.NewWorkerPool(3, handler, gliter.WithBuffer(6)) // make and spawn

	// - RUN -
	for range 10 {
		p.Push(0, 1, 2, 3)
		time.Sleep(100 * time.Millisecond)
		// periodically check/drain worker state
		results, errors := p.Collect()
		if errors != nil {
			panic(errors)
		}
		fmt.Println(results)
	}

	// - CLOSE -
	results, errors := p.Close().Collect() // waits for remaining work to complete
	if errors != nil {
		panic(errors)
	}
	fmt.Println(results)

}

func main() {
	fmt.Println("Worker pool simple:")
	simple()
	fmt.Println("Worker pool periodic:")
	periodic()
}
