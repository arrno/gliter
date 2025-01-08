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

// PLConfig controls limited pipeline behavior.
type PLConfig struct {
	LogAll      bool
	LogEmit     bool
	LogCount    bool
	ReturnCount bool
	LogStep     bool
}

type stage[T any] struct {
	stageType stageType
	handlers  []func(data T) (T, error)
}

// Pipeline spawns threads for all stage functions and orchestrates channel signals between them.
type Pipeline[T any] struct {
	generator func() (T, bool, error)
	stages    []stage[T]
	config    PLConfig
}

func NewPipeline[T any](gen func() (T, bool, error)) *Pipeline[T] {
	return &Pipeline[T]{
		generator: gen,
	}
}

// Stage pushes a new stage onto the pipeline. A stage should have > 0 transform functions. Each
// transform function beyond the first forks the pipeline into an additional downstream branch.
func (p *Pipeline[T]) Stage(fs []func(data T) (T, error)) *Pipeline[T] {
	st := stage[T]{
		stageType: FORK,
		handlers:  fs,
	}
	p.stages = append(p.stages, st)
	return p
}

// Throttle pushes a special throttle stage onto the pipeline. A throttle stage will merge all upstream
// branches into n downstream branches.
func (p *Pipeline[T]) Throttle(n uint) *Pipeline[T] {
	if n < 1 {
		n = 1
	}
	p.stages = append(p.stages, stage[T]{stageType: THROTTLE, handlers: make([]func(data T) (T, error), n)})
	return p
}

func (p *Pipeline[T]) Config(config PLConfig) *Pipeline[T] {
	p.config = config
	return p
}

func (p *Pipeline[T]) handleLog(val T) {
	if p.config.LogAll || p.config.LogEmit {
		fmt.Printf("Emit -> %v\n", val)
	}
}

func (p *Pipeline[T]) handleStageFunc(
	id string,
	inChan <-chan T,
	f func(data T) (T, error),
	done <-chan interface{},
	wg *sync.WaitGroup,
	stepSignal chan<- any,
	stepDone <-chan any,
) (outChan chan T, errChan chan error, node *PLNode[T]) {
	wg.Add(1)
	errChan = make(chan error)
	outChan = make(chan T)
	var val T
	node = NewPLNodeAs(id, val)
	keepCount := p.config.LogAll || p.config.LogCount || p.config.LogStep
	if stepDone != nil {
		done = Any(done, stepDone)
	}
	go func() {
		defer func() {
			close(outChan)
			close(errChan)
			wg.Done()
		}()
		for {
			if val, ok := ReadOrDone(inChan, done); ok {
				out, err := f(val)
				if keepCount {
					node.IncAs(out)
				}
				if stepSignal != nil {
					var T any
					if !WriteOrDone(T, stepSignal, done) {
						return
					}
				}
				if err != nil {
					if WriteOrDone(err, errChan, done) {
						continue
					} else {
						return
					}
				}
				if !WriteOrDone(out, outChan, done) {
					return
				}
			} else {
				return
			}
		}
	}()
	return
}

// Run builds and launches all the pipeline stages.
func (p *Pipeline[T]) Run() ([]PLNodeCount, error) {

	// Init logging helpers
	var val T
	root := NewPLNodeAs[T]("GEN", val)
	var stepSignal chan<- any
	var stepDone <-chan any
	if p.config.LogStep {
		stepSignal, stepDone = NewStepper(root).Run()
		defer close(stepSignal)
	}
	keepCount := p.config.LogAll || p.config.LogCount || p.config.LogStep || p.config.ReturnCount

	// Init async helpers
	done := make(chan any)
	var anyDone <-chan any
	anyDone = done
	if stepDone != nil {
		anyDone = Any(done, stepDone)
	}
	dataChan := make(chan T)
	var wg sync.WaitGroup

	errChan := make(chan error)
	// Init generator
	wg.Add(1)
	go func() {
		defer func() {
			close(errChan)
			close(dataChan)
			wg.Done()
		}()
		val, con, err := p.generator()
		if err != nil {
			WriteOrDone(err, errChan, done)
		}
		for con {
			if keepCount {
				root.IncAs(val)
			}
			if WriteOrDone(val, dataChan, anyDone) {
				val, con, err = p.generator()
				if err != nil {
					WriteOrDone(err, errChan, done)
				}
			} else {
				return
			}
		}
	}()

	// Stage async helpers
	errChans := List(errChan)
	prevOuts := List(dataChan)
	prevNodes := List(root)

	// Chain stages
	for idx, stage := range p.stages {
		if stage.stageType == THROTTLE {
			prevOuts = SliceToList(ThrottleBy(prevOuts.Iter(), anyDone, len(stage.handlers)))
			continue
		}

		outChans := List[chan T]()     // TODO optimize sizing
		outNodes := List[*PLNode[T]]() // TODO optimize sizing

		for j, po := range prevOuts.Iter() { // honor cumulative of prev forks
			parentNode := prevNodes.At(j)
			forkOut := TeeBy(po, anyDone, len(stage.handlers))
			for i, f := range stage.handlers { // for current stage
				outChan, errChan, node := p.handleStageFunc(
					fmt.Sprintf("%d:%d:%d", idx, j, i),
					forkOut[i],
					f,
					anyDone,
					&wg,
					stepSignal,
					stepDone,
				)
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
			if err, ok := ReadOrDone(errChan, anyDone); ok {
				if WriteOrDone(err, errBuff, anyDone) {
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
				if val, ok := ReadOrDone(prevOut, anyDone); ok {
					p.handleLog(val)
				} else {
					return
				}
			}
		}()
	}

	// Capture results
	wg.Wait()
	var err error
	select {
	case err = <-errBuff:
	default:
	}
	if p.config.LogAll || p.config.LogCount {
		root.PrintFullBF()
	}
	if p.config.ReturnCount {
		return root.CollectCount(), err
	}
	return nil, err
}
