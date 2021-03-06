package parallel

import "github.com/marcuswestin/fun-go/errs"

func Iterate(numItems int, maxParallelCount int, processingFunc func(i int) (err errs.Err)) (err errs.Err) {
	if numItems == 0 {
		return
	}
	if maxParallelCount > numItems {
		maxParallelCount = numItems
	}

	workChan := make(chan int)
	resultChan := make(chan errs.Err)
	errChan := make(chan errs.Err)
	go func() {
		// This func generates work
		index := 0
		workDone := 0
		// kick it off with N parallel executions
		for index < maxParallelCount {
			workChan <- index
			index += 1
		}
		// any time an execution is done, allow more work
		for index < numItems {
			if workErr := <-resultChan; workErr != nil {
				errChan <- workErr
				return
			}
			workDone += 1
			workChan <- index
			index += 1
		}
		for workDone < numItems {
			if workErr := <-resultChan; workErr != nil {
				errChan <- workErr
				return
			}
			workDone += 1
		}
		errChan <- nil
	}()

	go func() {
		for {
			go func(index int) {
				resultChan <- processingFunc(index)
			}(<-workChan)
		}
	}()

	return <-errChan
}
