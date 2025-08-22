package main

import "fmt"

func main() {
	fmt.Println("Example main:")
	ExampleMain()
	fmt.Println("Example fan out:")
	ExampleFanOut()

	// Worker pool examples
	fmt.Println("Example WP simple:")
	simple()
	fmt.Println("Example WP periodic:")
	periodic()
}
