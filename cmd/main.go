package main

import (
	"fmt"

	"github.com/arrno/glitter"
)

func main() {
	// push pop
	list := glitter.List(0, 1, 2, 3, 4)
	fmt.Println(list.Pop())
	list.Push(8)
	fmt.Println(list)
	// lambda
	newList := glitter.
		List(0, 1, 2, 3, 4).
		Filter(func(i int) bool { return i%2 == 0 }).
		Map(func(val int) int {
			return val * 2
		}).
		Reduce(func(acc *int, val int) {
			*acc += val
		})
	fmt.Println(*newList)

}
