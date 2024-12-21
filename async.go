package glitter

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
	for _ = range len(funcs) {
		select {
		case err := <-errChan:
			return nil, err
		case res := <-dataChan:
			results[res.order] = res.result
		}
	}
	return results, nil
}

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
					for _, ch := range outW {
						ch <- val
					}
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

func writeOrDone[T any](val T, write chan T, done chan any) bool {
	select {
	case write <- val:
		return true
	case <-done:
		return false
	}
}

func readOrDone[T any](read <-chan T, done chan any) (T, bool) {
	select {
	case val, ok := <-read:
		return val, ok
	case <-done:
		var zero T
		return zero, false
	}
}
