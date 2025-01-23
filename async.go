package gliter

import "sync"

// InParallel runs all the functions asynchronously and returns the results in order or the first error.
func InParallel[T any](funcs []func() (T, error)) ([]T, error) {
	type orderedResult struct {
		result T
		order  int
	}
	dataChan := make(chan orderedResult, len(funcs))
	errChan := make(chan error, len(funcs))
	for i, f := range funcs {
		go func(order int) {
			result, err := f()
			if err != nil {
				errChan <- err
			} else {
				dataChan <- orderedResult{result, order}
			}
		}(i)
	}
	results := make([]T, len(funcs))
	for range len(funcs) {
		select {
		case err := <-errChan:
			return nil, err
		case res := <-dataChan:
			results[res.order] = res.result
		}
	}
	return results, nil
}

// ThrottleBy merges the output of the provided channels into n output channels.
// This function returns when 'in' channels are closed or signal is received on 'done'.
func ThrottleBy[T any](in []chan T, done <-chan interface{}, n int) (out []chan T) {
	out = make([]chan T, n)
	for i := range n {
		out[i] = make(chan T)
	}

	go func() {
		defer func() {
			for _, ch := range out {
				close(ch)
			}
		}()
		var wg sync.WaitGroup
		// a signal on any inbound channel has equal chance or emitting on any outbound channel
		for _, inChan := range in {
			for _, outChan := range out {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						if val, ok := ReadOrDone(inChan, done); !ok {
							return
						} else if !WriteOrDone(val, outChan, done) {
							return
						}
					}
				}()
			}
		}
		wg.Wait()
	}()

	return
}

// TeeBy broadcasts all received signals on provided channel into n output channels.
// This function returns when 'in' channel is closed or signal is received on 'done'.
func TeeBy[T any](in <-chan T, done <-chan interface{}, n int) (outR []<-chan T) {
	if n < 2 {
		return []<-chan T{in}
	}
	outW := make([]chan T, n)
	outR = make([]<-chan T, n)
	for i := range n {
		ch := make(chan T)
		outW[i] = ch
		outR[i] = ch
	}

	go func() {
		defer func() {
			for _, ch := range outW {
				close(ch)
			}
		}()
		for {
			select {
			case val, ok := <-in:
				if ok {
					var wg sync.WaitGroup
					wg.Add(len(outW))
					for _, ch := range outW {
						go func() {
							defer wg.Done()
							WriteOrDone(val, ch, done)
						}()
					}
					wg.Wait()
				} else {
					return
				}
			case <-done:
				return
			}
		}
	}()

	return
}

// WriteOrDone blocks until it sends to 'write' or receives from 'done' and returns the boolean result.
func WriteOrDone[T any](val T, write chan<- T, done <-chan any) bool {
	select {
	case write <- val:
		return true
	case <-done:
		return false
	}
}

// ReadOrDone blocks until it receives from 'read' or receives from 'done' and returns the boolean result.
func ReadOrDone[T any](read <-chan T, done <-chan any) (T, bool) {
	select {
	case val, ok := <-read:
		return val, ok
	case <-done:
		var zero T
		return zero, false
	}
}

// Any consolidates a set of 'done' channels into one done channel.
func Any(channels ...<-chan any) <-chan any {
	switch len(channels) {
	case 0:
		return nil
	case 1:
		return channels[0]
	}
	orDone := make(chan any)
	go func() {
		defer close(orDone)
		switch len(channels) {
		case 2:
			select {
			case <-channels[0]:
			case <-channels[1]:
			}
		default:
			select {
			case <-channels[0]:
			case <-channels[1]:
			case <-channels[2]:
			case <-Any(append(channels[3:], orDone)...):
			}
		}
	}()
	return orDone
}

func IterDone[T any](read <-chan T, done <-chan any) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			val, ok := ReadOrDone(read, done)
			if !ok {
				return
			}
			out <- val
		}
	}()
	return out
}
