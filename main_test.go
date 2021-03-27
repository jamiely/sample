package main

import (
	"context"
	"sync"
	"testing"
)

func getTestToolOptions() *ToolOptions {
	workers := 2
	filename := "data/query_params.csv"

	ctx := context.Background()
	connStr := "none"
	workQueue := make(chan *HostFilter, 1)
	var waitGroup sync.WaitGroup

	toolOptions := ToolOptions{
		WorkerCount:      workers,
		ParamFilename:    filename,
		ConnectionString: connStr,
		Context:          &ctx,
		WorkerWaitGroup:  &waitGroup,
		HostFilterQueue:  workQueue,
		StatsQueue:       make(chan *Statistic, 5),
	}

	return &toolOptions
}

// We could have a property-based test here that ensures the
// worker index never goes out of bounds and that the distribution
// of indices is somewhat flat.
func Test_getWorkerIndex(t *testing.T) {
	opts := getTestToolOptions()
	opts.WorkerCount = 2

	testTable := map[string]int{
		"host1": 0,
		"host2": 1,
		"host3": 0,
	}

	for name, expected := range testTable {
		index := getWorkerIndex(opts, &HostFilter{name, "", "", 0})
		if index != expected {
			t.Errorf("getWorkerIndex(%s) = %d; want %d", name, index, expected)
		}
	}

}
