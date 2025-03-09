package gliter

import (
	"errors"
	"fmt"
	"sync"
)

type stageType int

const (
	FORK stageType = iota
	THROTTLE
	BATCH
	BUFFER
	MERGE
	OPTION
)

var (
	ErrNoGenerator     error = errors.New("Pipeline run error. Invalid pipeline: No generator provided")
	ErrEmptyStage      error = errors.New("Pipeline run error. Empty stage: No functions provided in stage")
	ErrEmptyThrottle   error = errors.New("Pipeline run error. Empty throttle: Throttle must be a positive value")
	ErrInvalidThrottle error = errors.New("Pipeline run error. Invalid throttle: Throttle value cannot be higher than channel count.")
	ErrNilFunc         error = errors.New("Pipeline run error. Invalid stage: Nil function.")
)

// PLConfig controls limited pipeline behavior.
type PLConfig struct {
	LogAll      bool
	LogEmit     bool
	LogCount    bool
	ReturnCount bool
	LogStep     bool
}

func (c PLConfig) keepCount() bool {
	return c.LogAll || c.LogCount || c.LogStep || c.ReturnCount
}

type stage[T any] struct {
	stageType stageType
	handlers  []func(T) (T, error)
}

type batch[T any] struct {
	stage   int
	size    int
	handler func([]T) ([]T, error)
}

// Pipeline spawns threads for all stage functions and orchestrates channel signals between them.
type Pipeline[T any] struct {
	generator   func() (T, bool, error)
	stages      []stage[T]
	batches     map[int]batch[T]
	merges      map[int]func([]T) ([]T, error)
	buffers     map[int]uint
	optionForks map[int][]func(T) (T, error)
	tally       <-chan int
	config      PLConfig
}

func NewPipeline[T any](gen func() (T, bool, error)) *Pipeline[T] {
	return &Pipeline[T]{
		batches:     map[int]batch[T]{},
		buffers:     map[int]uint{},
		merges:      map[int]func(data []T) ([]T, error){},
		optionForks: map[int][]func(T) (T, error){},
		generator:   gen,
	}
}

func (p *Pipeline[T]) Tally() chan<- int {
	tally := make(chan int)
	p.tally = tally
	return tally
}

// Stage pushes a new stage onto the pipeline. A stage should have > 0 transform functions. Each
// transform function beyond the first forks the pipeline into an additional downstream branch.
func (p *Pipeline[T]) Stage(fs ...func(data T) (T, error)) *Pipeline[T] {
	st := stage[T]{
		stageType: FORK,
		handlers:  fs,
	}
	p.stages = append(p.stages, st)
	return p
}

// Batch pushes a special batch stage onto the pipeline of size n. A batch allows you to operate on a set of items.
// This is helpful for expensive operations such as DB writes.
func (p *Pipeline[T]) Batch(n int, f func(set []T) ([]T, error)) *Pipeline[T] {
	p.batches[len(p.stages)] = batch[T]{stage: len(p.stages), size: n, handler: f}
	p.stages = append(p.stages, stage[T]{stageType: BATCH, handlers: make([]func(data T) (T, error), 1)})
	return p
}

// Buffer pushes a special buffer stage onto the pipeline of size n. A buffer stage is effectively a buffered channel of
// size n in between the previous and next stage.
func (p *Pipeline[T]) Buffer(n uint) *Pipeline[T] {
	p.buffers[len(p.stages)] = n
	p.stages = append(p.stages, stage[T]{stageType: BUFFER, handlers: make([]func(data T) (T, error), 1)})
	return p
}

// Throttle pushes a special throttle stage onto the pipeline. A throttle stage will converge all upstream
// branches into n downstream branches.
func (p *Pipeline[T]) Throttle(n uint) *Pipeline[T] {
	if n < 1 {
		n = 1
	}
	p.stages = append(p.stages, stage[T]{stageType: THROTTLE, handlers: make([]func(data T) (T, error), n)})
	return p
}

// Merge pushes a special merge stage onto the pipeline. A merge stage will merge all upstream
// branches into a single downstream branch with a merge function. Output is converted back into a stream.
func (p *Pipeline[T]) Merge(f func(data []T) ([]T, error)) *Pipeline[T] {
	p.merges[len(p.stages)] = f
	p.stages = append(p.stages, stage[T]{stageType: MERGE, handlers: make([]func(data T) (T, error), 1)})
	return p
}

// Option pushes a special option stage onto the pipeline. An option stage is effectively a buffer
// of size N where N is the number of handler functions. A record emitted from an upstream branch
// has an equal chance of entering any one of the option functions which are ready to receive.
func (p *Pipeline[T]) Option(fs ...func(data T) (T, error)) *Pipeline[T] {
	p.optionForks[len(p.stages)] = fs
	p.stages = append(p.stages, stage[T]{stageType: OPTION, handlers: make([]func(data T) (T, error), 1)})
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

func (p *Pipeline[T]) handleBufferFunc(
	id string,
	inChan <-chan T,
	index int,
	done <-chan interface{},
	wg *sync.WaitGroup,
	stepSignal chan<- any,
	stepDone <-chan any,
) (outChan chan T, errChan chan error, node *PLNode[T]) {

	wg.Add(1)
	errChan = make(chan error)
	outChan = make(chan T, p.buffers[index])
	var val T

	node = NewPLNodeAs(id, val)
	keepCount := p.config.keepCount()
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
			val, ok := ReadOrDone(inChan, done)
			if !ok {
				return
			}
			if keepCount {
				node.IncAs(val)
			}
			if stepSignal != nil {
				var T any
				if !WriteOrDone(T, stepSignal, done) {
					return
				}
			}
			if !WriteOrDone(val, outChan, done) {
				return
			}
		}
	}()

	return
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
	keepCount := p.config.keepCount()

	if stepDone != nil {
		done = Any(done, stepDone)
	}

	go func() {
		defer func() {
			close(outChan)
			close(errChan)
			wg.Done()
		}()

		if f == nil {
			WriteOrDone(ErrNilFunc, errChan, done)
			return
		}

		for {
			if val, ok := ReadOrDone(inChan, done); ok {
				out, err := f(val)
				if err != nil {
					WriteOrDone(err, errChan, done)
					return
				}
				if keepCount {
					node.IncAs(out)
				}
				if stepSignal != nil {
					var T any
					if !WriteOrDone(T, stepSignal, done) {
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

func (p *Pipeline[T]) handleBatchFunc(
	id string,
	inChan <-chan T,
	index int,
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
	keepCount := p.config.keepCount()

	if stepDone != nil {
		done = Any(done, stepDone)
	}

	batch := p.batches[index]
	queue := make([]T, 0, batch.size)

	handleBatch := func() bool {
		outSet, err := batch.handler(queue)
		if err != nil {
			WriteOrDone(err, errChan, done)
			return false
		}
		if keepCount {
			node.IncAsBatch(outSet)
		}
		if stepSignal != nil {
			var T any
			if !WriteOrDone(T, stepSignal, done) {
				return false
			}
		}
		for _, out := range outSet {
			if !WriteOrDone(out, outChan, done) {
				return false
			}
		}
		return true
	}

	go func() {
		defer func() {
			close(outChan)
			close(errChan)
			wg.Done()
		}()

		if batch.handler == nil {
			WriteOrDone(ErrNilFunc, errChan, done)
			return
		}

		for {
			if val, ok := ReadOrDone(inChan, done); ok {
				queue = append(queue, val)
				if len(queue) >= batch.size {
					if !handleBatch() {
						return
					}
					queue = nil
				}
			} else {
				if len(queue) > 0 && !handleBatch() {
					return
				}
				queue = nil
				return
			}
		}
	}()

	return
}

func (p *Pipeline[T]) handleOptionFunc(
	id string,
	inChan <-chan T,
	index int,
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
	keepCount := p.config.keepCount()
	if stepDone != nil {
		done = Any(done, stepDone)
	}

	optionFuncs := p.optionForks[index]

	buffer := make(chan T, len(optionFuncs))
	nilFunc := false
	var workerWg sync.WaitGroup

	// When all our workers exit, close downstream channels
	go func() {
		defer wg.Done()
		workerWg.Wait()
		close(outChan)
		close(errChan)
	}()

	for _, f := range optionFuncs {

		if f == nil {
			nilFunc = true
			break
		}
		workerWg.Add(1)

		// worker
		go func(handler func(T) (T, error)) {
			defer workerWg.Done()
			for {
				if val, ok := ReadOrDone(buffer, done); ok {
					result, err := handler(val)
					if err != nil {
						WriteOrDone(err, errChan, done)
						return
					}
					if keepCount {
						node.IncAsAtomic(result) // single node multi writers
					}
					if stepSignal != nil {
						var T any
						if !WriteOrDone(T, stepSignal, done) {
							return
						}
					}
					if !WriteOrDone(result, outChan, done) {
						return
					}
				} else {
					return
				}
			}
		}(f)
	}

	go func() {
		defer func() {
			close(buffer)
		}()

		if nilFunc {
			WriteOrDone(ErrNilFunc, errChan, done)
			return
		}

		for {
			if val, ok := ReadOrDone(inChan, done); ok {
				if !WriteOrDone(val, buffer, done) {
					return
				}
			} else {
				return
			}
		}
	}()

	return
}

func (p *Pipeline[T]) handleMergeFunc(
	id string,
	inChans []chan T,
	index int,
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
	keepCount := p.config.keepCount()
	if stepDone != nil {
		done = Any(done, stepDone)
	}

	mergeFunc := p.merges[index]
	queue := make([]T, 0, len(inChans))

	handleBatch := func() bool {
		outSet, err := mergeFunc(queue)
		if err != nil {
			WriteOrDone(err, errChan, done)
			return false
		}
		if keepCount {
			node.IncAsBatch(outSet)
		}
		if stepSignal != nil {
			var T any
			if !WriteOrDone(T, stepSignal, done) {
				return false
			}
		}
		for _, out := range outSet {
			if !WriteOrDone(out, outChan, done) {
				return false
			}
		}
		return true
	}

	go func() {
		defer func() {
			close(outChan)
			close(errChan)
			wg.Done()
		}()

		if mergeFunc == nil {
			WriteOrDone(ErrNilFunc, errChan, done)
			return
		}

		for {
			// read one per chan
			for _, inChan := range inChans {
				if val, ok := ReadOrDone(inChan, done); ok {
					queue = append(queue, val)
				} else {
					queue = nil
					return
				}
			}
			if !handleBatch() {
				return
			}
			queue = nil

		}
	}()

	return
}

// Run builds and launches all the pipeline stages.
func (p *Pipeline[T]) Run() ([]PLNodeCount, error) {

	if p.generator == nil {
		return nil, errors.New("pipeline run error. Invalid pipeline: No generator provided")
	}

	// Init logging helpers
	var val T
	root := NewPLNodeAs[T]("GEN", val)
	var stepSignal chan<- any
	var stepDone <-chan any

	if p.config.LogStep {
		stepSignal, stepDone = NewStepper(root).Run()
		defer close(stepSignal)
	}

	keepCount := p.config.keepCount()

	// Init async helpers
	done := make(chan any)
	var anyDone <-chan any
	anyDone = done

	if stepDone != nil {
		anyDone = Any(done, stepDone)
	}

	dataChan := make(chan T)
	errChan := make(chan error)
	var wg sync.WaitGroup

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
		if len(stage.handlers) == 0 {
			continue
		}

		size := prevOuts.Len() * len(stage.handlers)
		outChans := MakeList[chan T](0, uint(size))
		outNodes := MakeList[*PLNode[T]](0, uint(size))

		if stage.stageType == THROTTLE {
			// Invalid throttle length
			if len(stage.handlers) > prevOuts.Len() {
				throttleErr := make(chan error, 1)
				throttleErr <- ErrInvalidThrottle
				errChans.Push(throttleErr)
				break
			}

			// child nodes of throttle don't exactly line up as you would expect in node tree
			// that's ok so long as throttle size is smaller than previous stage.
			// PLNode tree is just for logging.
			prevOuts = SliceToList(ThrottleBy(prevOuts.Iter(), anyDone, len(stage.handlers)))
			for i := range len(stage.handlers) {
				node := NewPLNodeAs("[THROTTLE]", val)
				node.encap = true
				prevNodes.At(i).SpawnAs(node)
				outNodes.Push(node)
			}

			prevNodes = outNodes
			continue
		}

		if stage.stageType == MERGE {
			outChan, errChan, node := p.handleMergeFunc(
				"[MERGE]",
				prevOuts.Unwrap(),
				idx,
				anyDone,
				&wg,
				stepSignal,
				stepDone,
			)

			prevNodes.At(0).SpawnAs(node)
			errChans.Push(errChan)
			prevOuts = List(outChan)
			prevNodes = List(node)
			continue
		}

		for j, po := range prevOuts.Iter() { // honor cumulative of prev forks
			parentNode := prevNodes.At(j)
			forkOut := TeeBy(po, anyDone, len(stage.handlers))

			for i, f := range stage.handlers { // for current stage
				// var outChan, errChan, node
				var outChan chan T
				var errChan chan error
				var node *PLNode[T]

				if stage.stageType == BUFFER {
					outChan, errChan, node = p.handleBufferFunc(
						"[BUFFER]",
						forkOut[i],
						idx,
						anyDone,
						&wg,
						stepSignal,
						stepDone,
					)
				} else if stage.stageType == BATCH {
					outChan, errChan, node = p.handleBatchFunc(
						fmt.Sprintf("%d:%d:%d", idx, j, i),
						forkOut[i],
						idx,
						anyDone,
						&wg,
						stepSignal,
						stepDone,
					)
				} else if stage.stageType == OPTION {
					outChan, errChan, node = p.handleOptionFunc(
						fmt.Sprintf("%d:%d:%d", idx, j, i),
						forkOut[i],
						idx,
						anyDone,
						&wg,
						stepSignal,
						stepDone,
					)
				} else {
					outChan, errChan, node = p.handleStageFunc(
						fmt.Sprintf("%d:%d:%d", idx, j, i),
						forkOut[i],
						f,
						anyDone,
						&wg,
						stepSignal,
						stepDone,
					)
				}

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

	// Conditional tally
	var tallyChan chan int
	var tallyDone chan any // tally needs to stay open until all stages exit

	// if tally was created
	if p.tally != nil {
		tallyChan = make(chan int, 1)
		tallyDone = make(chan any)
		go func() {
			// anyAnyDone := Any(anyDone, tallyDone)
			tallyCount := 0
			defer func() { tallyChan <- tallyCount }()
			for {
				if val, ok := ReadOrDone(p.tally, tallyDone); ok {
					tallyCount += val
				} else {
					return
				}
			}
		}()
	}

	// Capture, display/return results
	tallyResult := 0
	wg.Wait() // wait for all stages to exit

	if p.tally != nil {
		close(tallyDone)
		tallyResult = <-tallyChan
	}

	var err error
	select {
	case err = <-errBuff:
	default:
	}

	if p.config.LogAll || p.config.LogCount {
		root.PrintFullBF()
	}
	if p.tally != nil {
		return []PLNodeCount{{NodeID: "tally", Count: tallyResult}}, err
	}
	if p.config.ReturnCount {
		return root.CollectCount(), err
	}

	return nil, err
}
