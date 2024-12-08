package main

import (
	"fmt"

	"github.com/arrno/glitter"
)

func main() {
	// push pop
	list := glitter.IntoList([]uint{0, 1, 2, 3, 4})
	fmt.Println(list.Pop())
	list.Push(8)
	fmt.Println(list)
	// lambda
	newList := glitter.
		IntoList([]uint{0, 1, 2, 3, 4}).
		Filter(func(i uint) bool { return i%2 == 0 }).
		Map(func(val uint) uint {
			return val * 2
		}).
		Reduce(func(acc *uint, val uint) {
			*acc += val
		})
	fmt.Println(*newList)

}
