package gliter

import (
	"fmt"
	"sync"
)

type stageType int

const (
	FORK stageType = iota
	CONVERGE
)

type PLConfig struct {
	Log bool
}

type stage[T any] struct {
	stageType stageType
	handlers  []func(data T) (T, error)
}

type Pipeline[T any] struct {
	generator func() (T, bool)
	stages    []stage[T]
	config    PLConfig
}

func NewPipeline[T any](gen func() (T, bool)) *Pipeline[T] {
	return &Pipeline[T]{
		generator: gen,
	}
}

func (p *Pipeline[T]) Stage(fs []func(data T) (T, error)) *Pipeline[T] {
	st := stage[T]{
		stageType: FORK,
		handlers:  fs,
	}
	p.stages = append(p.stages, st)
	return p
}

// TODO honor me
// func (p *Pipeline[T]) Converge() *Pipeline[T] {
// 	p.stages = append(p.stages, stage[T]{stageType: CONVERGE})
// 	return p
// }

func (p *Pipeline[T]) Config(config PLConfig) *Pipeline[T] {
	p.config = config
	return p
}

func (p *Pipeline[T]) handleLog(val T) {
	if p.config.Log {
		fmt.Println(val)
	}
}

func (p *Pipeline[T]) Run() error {
	done := make(chan interface{})
	dataChan := make(chan T)
	var wg sync.WaitGroup
	wg.Add(1)
	// Init generator
	go func() {
		defer func() {
			close(dataChan)
			wg.Done()
		}()
		val, con := p.generator()
		for con {
			if writeOrDone(val, dataChan, done) {
				val, con = p.generator()
			} else {
				return
			}
		}
	}()
	errChans := List[chan error]()
	prevOuts := List(dataChan)
	// Chain stages
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
					if val, ok := readOrDone(inChan, done); ok {
						out, err := f(val)
						if err != nil {
							if writeOrDone(err, errChan, done) {
								continue
							} else {
								return
							}
						}
						if !writeOrDone(out, outChan, done) {
							return
						}
					} else {
						return
					}
				}
			}()
			return
		}
		outChans := List[chan T]()

		for _, po := range prevOuts.Iter() { // honor cumulative of prev forks
			forkOut := TeeBy(po, done, len(stage.handlers))
			for i, f := range stage.handlers { // for current stage
				outChan, errChan := do(forkOut[i], f)
				errChans.Push(errChan)
				outChans.Push(outChan)
			}
		}
		prevOuts = outChans
	}
	// Listen for errors
	errBuff := make(chan error, 1)
	for _, errChan := range errChans.Iter() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err, ok := readOrDone(errChan, done); ok {
				if writeOrDone(err, errBuff, done) {
					// Will only reach once since err buffer has cap of 1
					close(done)
				}
			}
		}()
	}
	// Drain end of pipeline
	for _, prevOut := range prevOuts.Iter() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if val, ok := readOrDone(prevOut, done); ok {
					p.handleLog(val)
				} else {
					return
				}
			}
		}()
	}
	wg.Wait()
	var err error
	select {
	case err = <-errBuff:
	default:
	}
	return err
}
