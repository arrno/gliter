package glitter

import (
	"fmt"
	"sync"
)

/*
TODO:
listen for errors
Return something useful
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
	errChans := List[chan error]()
	prevOuts := List(dataChan)
	for _, stage := range p.stages {
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
		outChans := List[chan T]()

		for _, po := range prevOuts.Iter() { // honor cum sum of prev forks
			forkOut := TeeBy(po, done, len(stage))
			for i, f := range stage { // for current stage
				outChan, errChan := do(forkOut[i], f)
				errChans.Push(errChan)
				outChans.Push(outChan)
			}
		}
		prevOuts = outChans
	}
	// Drain end of pipeline
	for _, prevOut := range prevOuts.Iter() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case val, ok := <-prevOut:
					if !ok {
						return
					}
					fmt.Println(val)
				}
			}
		}()
	}
	wg.Wait()
}
