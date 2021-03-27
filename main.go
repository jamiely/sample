package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"sync"

	"github.com/jackc/pgx/v4"
)

type HostFilter struct {
	Host          string
	StartDateTime string
	EndDateTime   string
	LineNumber    int
}

type ToolOptions struct {
	WorkerCount      int
	ConnectionString string
	Context          *context.Context
	WaitGroup        *sync.WaitGroup
	WorkQueue        chan *HostFilter
}

type WorkerDef struct {
	Id        int
	WorkQueue chan *HostFilter
}

func runQuery(opts *ToolOptions, workerDef *WorkerDef, hostFilter *HostFilter) {
	conn, err := pgx.Connect(*opts.Context, opts.ConnectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(*opts.Context)

	rows, err := conn.Query(*opts.Context,
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
		hostFilter.Host,
		hostFilter.StartDateTime,
		hostFilter.EndDateTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	rowCount := 0
	for rows.Next() {
		rowCount++
	}

	log.Println("Worker", workerDef.Id, "Line", hostFilter.LineNumber, "Host",
		hostFilter.Host, ": Got", rowCount, "rows", hostFilter.EndDateTime)
}

func runWorker(opts *ToolOptions, workerDef *WorkerDef) {
	log.Println("Started worker", workerDef.Id)
	defer opts.WaitGroup.Done()

	for hostFilter := range workerDef.WorkQueue {
		runQuery(opts, workerDef, hostFilter)
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

	log.Println("Parsing file", filename)

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
			fmt.Fprint(os.Stderr, "Invalid row on line", line)
			continue
		}

		workQueue <- &HostFilter{record[0], record[1], record[2], line}

		line++
	}

	log.Println("Read", line, "lines from input")

	close(workQueue)
}

//connect to database using a single connection
func main() {
	workers := flag.Int("workers", 1, "The number of workers to use")
	filename := flag.String("params-file", "data/query_params.csv", "The file to load query params from")

	flag.Parse()

	file, err := os.OpenFile("timescale_benchmark.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	log.Println("Starting with", *workers, "workers")
	log.Println("Starting with", *filename, "filename")
	/***********************************************/
	/* Single Connection to TimescaleDB/ PostresQL */
	/***********************************************/
	ctx := context.Background()
	// TODO: move to environment var
	connStr := "postgresql://postgres:password@localhost:5432/homework"
	workQueue := make(chan *HostFilter, *workers*2)
	var waitGroup sync.WaitGroup

	toolOptions := ToolOptions{*workers, connStr, &ctx, &waitGroup, workQueue}

	dispatchWork(&toolOptions, *workers)
	fillWorkQueue(*filename, workQueue)
	waitGroup.Wait()
}

func hashHostFilter(opts *ToolOptions, hostFilter *HostFilter) int {
	h := fnv.New32a()
	h.Write([]byte(hostFilter.Host))
	return int(h.Sum32()) % opts.WorkerCount
}

func dispatchWork(opts *ToolOptions, workerCount int) {
	// we want queries for a particular host to always
	// be queried by the same worker, so we will use
	// the hash value of the host name to assign to
	// different workers. Each worker will have their
	// own channel.
	var workers []*WorkerDef
	for i := 0; i < workerCount; i++ {
		workers = append(workers, &WorkerDef{
			Id:        i + 1,
			WorkQueue: make(chan *HostFilter)})
	}

	for _, worker := range workers {
		opts.WaitGroup.Add(1)
		go runWorker(opts, worker)
	}

	go func() {
		for filter := range opts.WorkQueue {
			hashValue := hashHostFilter(opts, filter)
			workers[hashValue].WorkQueue <- filter
		}

		for _, worker := range workers {
			close(worker.WorkQueue)
		}
	}()
}
