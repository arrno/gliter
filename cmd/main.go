package main

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/arrno/gliter"
)

func main() {
	// MainPipeline()
	// fmt.Println(rand.Float64() * 100)
	fmt.Println(math.Round(rand.Float64()*10000) / 100)
}

func MainPipeline() {
	gliter.NewPipeline(exampleGen()).
		Config(gliter.PLConfig{LogStep: true}).
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

func exampleGen() func() (int, bool, error) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (int, bool, error) {
		index++
		if index == len(data) {
			return 0, false, nil
		}
		return data[index], true, nil
	}
}

func exampleMid(i int) (int, error) {
	return i * 2, nil
}

func exampleEnd(i int) (int, error) {
	return i * i, nil
}
