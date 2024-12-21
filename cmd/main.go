package main

import (
	"github.com/arrno/glitter"
)

func main() {
	// // push pop
	// list := glitter.List(0, 1, 2, 3, 4)
	// fmt.Println(list.Pop())
	// list.Push(8)
	// fmt.Println(list)
	// // lambda
	// newList := glitter.
	// 	List(0, 1, 2, 3, 4).
	// 	Filter(func(i int) bool { return i%2 == 0 }).
	// 	Map(func(val int) int {
	// 		return val * 2
	// 	}).
	// 	Reduce(func(acc *int, val int) {
	// 		*acc += val
	// 	})
	// fmt.Println(*newList)

	MainPipeline()

}

func MainPipeline() {
	glitter.NewPipeline(example_gen()).
		Config(glitter.PLConfig{Log: true}).
		Stage(
			[]func(i int) (int, error){
				example_mid, // branch A
				example_mid, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				example_end,
			},
		).
		Run()
}

func example_gen() func() (int, bool) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool) {
		index++
		if index == len(data) {
			return 0, false
		}
		return data[index], true
	}
}

func example_mid(i int) (int, error) {
	return i * 2, nil
}

func example_end(i int) (int, error) {
	return i * i, nil
}
