package gliter

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
)

// Stepper provides a central point to block all threads and await user intervention.
type Stepper[T any] struct {
	signal <-chan any
	root   *PLNode[T]
}

func NewStepper[T any](root *PLNode[T]) *Stepper[T] {
	s := new(Stepper[T])
	s.root = root
	return s
}

// Run launches a stepper thread and provides a signal channel for external threads to write to.
// If this function exists, the returned 'done' channel is closed.
func (s *Stepper[T]) Run() (chan<- any, <-chan any) {
	signal := make(chan any)
	done := make(chan any)
	s.signal = signal
	go func() {
		defer close(done)
		for range s.signal {
			s.clearConsole()
			s.root.PrintFullBF()
			fmt.Print("Step [Y/n]:")
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

func (s *Stepper[T]) clearConsole() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
