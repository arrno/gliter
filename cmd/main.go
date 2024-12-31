package main

import (
	"time"

	"github.com/arrno/gliter"
)

func main() {
	MainPipeline()
}

func MainPipeline() {
	gliter.NewPipeline(exampleGen()).
		Config(gliter.PLConfig{Log: true}).
		Stage(
			[]func(val map[string]any) (map[string]any, error){
				exampleNice,
				exampleMean,
			},
		).
		Stage(
			[]func(val map[string]any) (map[string]any, error){
				exampleEnd,
			},
		).
		Run()
}

func exampleGen() func() (map[string]any, bool) {
	data := []int{1, 2, 3, 4, 5}
	index := -1
	return func() (map[string]any, bool) {
		index++
		if index == len(data) {
			return nil, false
		}
		return map[string]any{"Hello": data[index]}, true
	}
}

func exampleNice(val map[string]any) (map[string]any, error) {
	val["NICE!!!"] = "!"
	return val, nil
}
func exampleMean(val map[string]any) (map[string]any, error) {
	time.Sleep(time.Duration(1) * time.Second)
	val["MEAN???"] = "!!"
	delete(val, "NICE!!!")
	return val, nil
}

func exampleEnd(val map[string]any) (map[string]any, error) {
	return val, nil
}
