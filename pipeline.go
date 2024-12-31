package gliter

import (
	"fmt"
	"sync"
)

type stageType int

const (
	FORK stageType = iota
	THROTTLE
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
	tree      PLNode[T]
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

func (p *Pipeline[T]) Throttle(by uint) *Pipeline[T] {
	if by < 1 {
		by = 1
	}
	p.stages = append(p.stages, stage[T]{stageType: THROTTLE, handlers: make([]func(data T) (T, error), by)})
	return p
}

func (p *Pipeline[T]) Config(config PLConfig) *Pipeline[T] {
	p.config = config
	return p
}

func (p *Pipeline[T]) handleLog(val T) {
	if p.config.Log {
		fmt.Printf("Emit -> %v\n", val)
	}
}

func (p *Pipeline[T]) handleStageFunc(id string, inChan <-chan T, f func(data T) (T, error), done <-chan interface{}, wg *sync.WaitGroup) (outChan chan T, errChan chan error, node *PLNode[T]) {
	wg.Add(1)
	errChan = make(chan error)
	outChan = make(chan T)
	var val T
	node = NewPLNodeAs(id, val)
	go func() {
		defer func() {
			close(outChan)
			close(errChan)
			wg.Done()
		}()
		for {
			if val, ok := readOrDone(inChan, done); ok {
				out, err := f(val)
				node.IncAs(out)
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

func (p *Pipeline[T]) Run() error {
	done := make(chan interface{})
	dataChan := make(chan T)
	var wg sync.WaitGroup
	wg.Add(1)
	// Init generator
	var val T
	root := NewPLNodeAs[T]("GEN", val)
	go func() {
		defer func() {
			close(dataChan)
			wg.Done()
		}()
		val, con := p.generator()
		for con {
			root.IncAs(val)
			if writeOrDone(val, dataChan, done) {
				val, con = p.generator()
			} else {
				return
			}
		}
	}()
	errChans := List[chan error]()
	prevOuts := List(dataChan)
	prevNodes := List(root)
	// Chain stages
	for idx, stage := range p.stages {
		if stage.stageType == THROTTLE {
			prevOuts = SliceToList(ThrottleBy(prevOuts.Iter(), done, len(stage.handlers)))
			continue
		}

		outChans := List[chan T]()     // TODO optimize sizing
		outNodes := List[*PLNode[T]]() // TODO optimize sizing

		for j, po := range prevOuts.Iter() { // honor cumulative of prev forks
			parentNode := prevNodes.At(j)
			forkOut := TeeBy(po, done, len(stage.handlers))
			for i, f := range stage.handlers { // for current stage
				outChan, errChan, node := p.handleStageFunc(fmt.Sprintf("%d:%d:%d", idx, j, i), forkOut[i], f, done, &wg)
				errChans.Push(errChan)
				outChans.Push(outChan)
				outNodes.Push(node)
				parentNode.SpawnAs(node)
			}
		}
		prevOuts = outChans
		prevNodes = outNodes
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
	if p.config.Log {
		root.PrintFullBF()
	}
	return err
}
