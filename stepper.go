package gliter

import (
	"fmt"
	"slices"
	"strings"
)

type Stepper[T any] struct {
	signal <-chan any
	root   *PLNode[T]
}

func NewStepper[T any](root *PLNode[T]) *Stepper[T] {
	s := new(Stepper[T])
	s.root = root
	return s
}

func (s *Stepper[T]) Run() (chan<- any, <-chan any) {
	signal := make(chan any)
	done := make(chan any)
	s.signal = signal
	go func() {
		defer close(done)
		for range s.signal {
			s.root.PrintFullBF()
			fmt.Print("Step [Y/n]: ")
			var input string
			fmt.Scanln(&input)
			if slices.Contains([]string{"", "y"}, strings.ToLower(input)) {
				continue
			}
			return
		}
	}()
	return signal, done
}
