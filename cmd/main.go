package main

import "github.com/arrno/gliter"

func main() {
	MainPipeline()
}

func MainPipeline() {
	gliter.NewPipeline(exampleGen()).
		Config(gliter.PLConfig{Log: true}).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branch A
				exampleMid, // branch B
			},
		).
		Stage(
			[]func(i int) (int, error){
				exampleMid, // branches A.C, B.C
				exampleMid, // branches A.D, B.D
				exampleMid, // branches A.E, B.E
			},
		).
		Throttle(2). // merge into branches X, Z
		Stage(
			[]func(i int) (int, error){
				exampleEnd,
			},
		).
		Run()
}

func exampleGen() func() (int, bool) {
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

func exampleMid(i int) (int, error) {
	return i * 2, nil
}

func exampleEnd(i int) (int, error) {
	return i * i, nil
}
