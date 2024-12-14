package glitter

import (
	"fmt"
	"sync"
)

/*
TODO:
listen for errors
Return something useful
Stage forking
*/

type stage[T any] []func(data T) (T, error)

type Pipeline[T any] struct {
	generator func() (T, bool)
	stages    []stage[T]
}

func NewPipeline[T any](gen func() (T, bool)) *Pipeline[T] {
	return &Pipeline[T]{
		generator: gen,
	}
}

func (p *Pipeline[T]) Stage(st stage[T]) *Pipeline[T] {
	p.stages = append(p.stages, st)
	return p
}

func (p *Pipeline[T]) Run() {
	done := make(chan interface{})
	dataChan := make(chan T)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			close(dataChan)
			wg.Done()
		}()
		val, con := p.generator()
		for con {
			select {
			case dataChan <- val:
				val, con = p.generator()
			case <-done:
				return
			}
		}
	}()
	errChans := make([]chan error, len(p.stages))
	prevOut := dataChan
	for i, stage := range p.stages {
		do := func(inChan <-chan T, f func(data T) (T, error)) (outChan chan T, errChan chan error) {
			wg.Add(1)
			errChan = make(chan error)
			outChan = make(chan T)
			go func() {
				defer func() {
					close(outChan)
					close(errChan)
					wg.Done()
				}()
				for {
					select {
					case <-done:
						return
					case val, ok := <-inChan:
						if !ok {
							return
						}
						out, err := f(val)
						if err != nil {
							select {
							case <-done:
								return
							case errChan <- err:
							}
						}
						select {
						case <-done:
							return
						case outChan <- out:
						}
					}
				}
			}()
			return
		}
		outChan, errChan := do(prevOut, stage[0]) // for now no forking
		errChans[i] = errChan
		prevOut = outChan
	}
	// Drain end of pipeline
	go func() {
		for {
			select {
			case <-done:
				return
			case val := <-prevOut:
				fmt.Println(val)
			}
		}
	}()
	wg.Wait()
}
