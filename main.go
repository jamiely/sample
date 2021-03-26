package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/jackc/pgx/v4"
)

type HostFilter struct {
	host          string
	startDateTime string
	endDateTime   string
	lineNumber    int
}

func runQuery(ctx *context.Context, connStr string, hostFilter *HostFilter) {
	conn, err := pgx.Connect(*ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(*ctx)

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

	fmt.Println(hostFilter.lineNumber, ": Got", rowCount, "rows", hostFilter.endDateTime)
}

func runWorker(ctx *context.Context, connStr string, waitGroup *sync.WaitGroup, workQueue chan *HostFilter) {
	fmt.Println("Started worker")
	defer waitGroup.Done()

	for hostFilter := range workQueue {
		runQuery(ctx, connStr, hostFilter)
	}

}

func getFile(filename string) (*os.File, error) {
	if filename == "-" {
		return os.Stdin, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open")
		return nil, err
	}
	return file, nil
}

func fillWorkQueue(filename string, workQueue chan<- *HostFilter) {
	file, err := getFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problem reading file")
		os.Exit(1)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	csvReader := csv.NewReader(reader)

	fmt.Println("Parsing file")

	line := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if line == 1 {
			// header
			line++
			continue
		}

		if len(record) != 3 {
			fmt.Fprintf(os.Stderr, "Invalid row")
			continue
		}

		workQueue <- &HostFilter{record[0], record[1], record[2], line}

		line++
	}

	close(workQueue)
}

//connect to database using a single connection
func main() {
	workers := flag.Int("workers", 1, "The number of workers to use")
	filename := flag.String("params-file", "data/query_params.csv", "The file to load query params from")

	flag.Parse()
	fmt.Println("Starting with", *workers, "workers")
	fmt.Println("Starting with", *filename, "filename")
	/***********************************************/
	/* Single Connection to TimescaleDB/ PostresQL */
	/***********************************************/
	ctx := context.Background()
	// TODO: move to environment var
	connStr := "postgresql://postgres:password@localhost:5432/homework"

	//run a simple query to check our connection
	var waitGroup sync.WaitGroup
	workQueue := make(chan *HostFilter, 5)

	for i := 0; i < *workers; i++ {
		waitGroup.Add(1)
		fmt.Println("Running goroutine")
		go runWorker(&ctx, connStr, &waitGroup, workQueue)
	}

	fillWorkQueue(*filename, workQueue)
	waitGroup.Wait()
}
