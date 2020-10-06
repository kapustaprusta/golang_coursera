package main

import (
	"fmt"
	"sort"
	"sync"
)

const (
	MD5CalculatorLimit = 1
	ThLimit            = 6
)

// func ExecutePipeline ...
func ExecutePipeline(jobs ...job) {
	// make pipes
	pipes := make([]chan interface{}, len(jobs)+1)
	for pipeIdx := 0; pipeIdx < len(pipes); pipeIdx++ {
		pipes[pipeIdx] = make(chan interface{}, 100)
	}

	// launch jobs
	var wg sync.WaitGroup
	for currJobIdx, currJob := range jobs {
		wg.Add(1)
		go func(currJobIdx int, currJob job) {
			defer wg.Done()
			currJob(pipes[currJobIdx], pipes[currJobIdx+1])
			close(pipes[currJobIdx+1])
		}(currJobIdx, currJob)
	}
	wg.Wait()
}

// func SingleHash ...
func SingleHash(in, out chan interface{}) {
	var outerWG sync.WaitGroup
	quotaCh := make(chan struct{}, MD5CalculatorLimit)

	for rawData := range in {
		outerWG.Add(1)
		go func(rawData interface{}) {
			defer outerWG.Done()

			waitCh := make(chan struct{}, 1)
			leftSideHashSum := ""
			go func() {
				waitCh <- struct{}{}
				leftSideHashSum = DataSignerCrc32(fmt.Sprint(rawData))
				<-waitCh
			}()

			// limited resource
			quotaCh <- struct{}{}
			tmpMd5HashSum := DataSignerMd5(fmt.Sprint(rawData))
			<-quotaCh
			//
			rightSideHashSum := DataSignerCrc32(tmpMd5HashSum)

			waitCh <- struct{}{}
			out <- leftSideHashSum + "~" + rightSideHashSum
			<-waitCh
		}(rawData)
	}
	outerWG.Wait()
}

// func MultiHash ...
func MultiHash(in, out chan interface{}) {
	var outerWG sync.WaitGroup

	for rawData := range in {
		outerWG.Add(1)
		go func(rawData interface{}) {
			defer outerWG.Done()

			results := make([]string, ThLimit)
			var innerWG sync.WaitGroup

			for th := 0; th < ThLimit; th++ {
				innerWG.Add(1)
				go func(th int) {
					defer innerWG.Done()
					results[th] = DataSignerCrc32(fmt.Sprint(th) + fmt.Sprint(rawData))
				}(th)
			}
			innerWG.Wait()

			resultsStr := ""
			for _, result := range results {
				resultsStr += result
			}

			out <- resultsStr
		}(rawData)
	}

	outerWG.Wait()
}

// func CombineResults ...
func CombineResults(in, out chan interface{}) {
	results := make([]string, len(in))

	for rawData := range in {
		results = append(results, fmt.Sprint(rawData))
	}

	combinedResults := ""
	sort.Strings(results)
	for hashSumIdx, hashSum := range results {
		combinedResults += hashSum
		if hashSumIdx < len(results)-1 {
			combinedResults += "_"
		}
	}

	out <- combinedResults
}
