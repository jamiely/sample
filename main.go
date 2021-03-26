package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/jackc/pgx/v4"
)

type HostFilter struct {
	host          string
	startDateTime string
	endDateTime   string
}

func runQuery(ctx *context.Context, connStr string, hostFilter *HostFilter) {
	conn, err := pgx.Connect(*ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	rows, err := conn.Query(*ctx,
		`select 
            host, 
            time_bucket_gapfill('1 minute', ts) as onemin, 
            min(usage), 
            max(usage) 
            from cpu_usage 
            where 
                host = $1
                and ts between $2
                            and $3
            group by host, onemin 
            order by onemin;`,
		hostFilter.host,
		hostFilter.startDateTime,
		hostFilter.endDateTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	rowCount := 0
	for rows.Next() {
		rowCount++
	}
	conn.Close(*ctx)

	fmt.Println("Got", rowCount, "rows")
}

func runWorker(ctx *context.Context, connStr string, waitGroup *sync.WaitGroup, workQueue chan *HostFilter) {
	fmt.Println("Started worker")
	defer waitGroup.Done()
	for {
		select {
		case hostFilter := <-workQueue:
			runQuery(ctx, connStr, hostFilter)
		default:
			return
		}
	}
}

//connect to database using a single connection
func main() {
	workers := flag.Int("workers", 1, "The number of workers to use")
	flag.Parse()
	fmt.Println("Starting with", *workers, "workers")
	/***********************************************/
	/* Single Connection to TimescaleDB/ PostresQL */
	/***********************************************/
	ctx := context.Background()
	// TODO: move to environment var
	connStr := "postgresql://postgres:password@localhost:5432/homework"

	//run a simple query to check our connection
	var waitGroup sync.WaitGroup
	hostFilters := []HostFilter{
		{"host_000008", "2017-01-01 08:59:22", "2017-01-01 09:59:22"},
		{"host_000008", "2017-01-01 08:59:22", "2017-01-01 09:59:22"},
	}

	workQueue := make(chan *HostFilter, 5)

	for i := range hostFilters {
		workQueue <- &hostFilters[i]
	}

	for i := 0; i < *workers; i++ {
		waitGroup.Add(1)
		fmt.Println("Running goroutine")
		go runWorker(&ctx, connStr, &waitGroup, workQueue)
	}
	waitGroup.Wait()
}
