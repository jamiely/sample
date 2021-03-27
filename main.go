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
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/montanaflynn/stats"
)

type HostFilter struct {
	Host          string
	StartDateTime string
	EndDateTime   string
	LineNumber    int
}

type ToolOptions struct {
	WorkerCount      int
	ParamFilename    string
	ConnectionString string
	Context          *context.Context
	WorkerWaitGroup  *sync.WaitGroup
	HostFilterQueue  chan *HostFilter
	StatsQueue       chan *Statistic
}

type WorkerDef struct {
	Id              int
	HostFilterQueue chan *HostFilter
}

type Statistic struct {
	IsError       bool
	QueryDuration int64
}

// don't modify this!
var StatisticError = Statistic{
	IsError:       true,
	QueryDuration: 0,
}

// Runs a query against the database based on the passed
// host filter.
func runQuery(opts *ToolOptions, workerDef *WorkerDef, hostFilter *HostFilter) error {
	conn, err := pgx.Connect(*opts.Context, opts.ConnectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		return err
	}
	defer conn.Close(*opts.Context)

	start := time.Now()
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
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		return err
	}
	opts.StatsQueue <- &Statistic{
		IsError:       false,
		QueryDuration: int64(elapsed)}

	rowCount := 0
	for rows.Next() {
		rowCount++
	}

	log.Println("Worker", workerDef.Id, "Line", hostFilter.LineNumber, "Host",
		hostFilter.Host, ": Got", rowCount, "rows", hostFilter.EndDateTime)

	return nil
}

// Runs the worker defined by the passed worker definition.
// The worker will read all tasks from its own work queue
// until close.
func runWorker(opts *ToolOptions, workerDef *WorkerDef) {
	log.Println("Started worker", workerDef.Id)
	defer opts.WorkerWaitGroup.Done()

	for hostFilter := range workerDef.HostFilterQueue {
		err := runQuery(opts, workerDef, hostFilter)
		if err != nil {
			opts.StatsQueue <- &StatisticError
		}
	}

}

// Retrieves the file to use. If `-` is given,
// we will load data from stdin.
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

// Fills the primary work queue by reading the params
// file, marshalling those params into host filters,
// and sending them to the channel.
func fillWorkQueue(opts *ToolOptions) {
	file, err := getFile(opts.ParamFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Problem reading file")
		os.Exit(1)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	csvReader := csv.NewReader(reader)

	log.Println("Parsing file", opts.ParamFilename)

	line := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if line == 1 {
			// we always assume the first line will be a header.
			line++
			continue
		}

		if len(record) != 3 {
			fmt.Fprint(os.Stderr, "Invalid row on line", line)
			continue
		}

		opts.HostFilterQueue <- &HostFilter{
			Host:          record[0],
			StartDateTime: record[1],
			EndDateTime:   record[2],
			LineNumber:    line,
		}

		line++
	}

	log.Println("Read", line, "lines from input")

	close(opts.HostFilterQueue)
}

// Loads the main context of the program including command
// line arguments and environment variables.
func loadToolOptions() *ToolOptions {
	workers := flag.Int("workers", 1, "The number of workers to use")
	filename := flag.String("params-file", "data/query_params.csv", "The file to load query params from")
	// showHelp := flag.Bool("help", false, "Show help")

	flag.Parse()

	file, err := os.OpenFile("logs/timescale_benchmark.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	log.Println("Starting with", *workers, "workers")
	log.Println("Starting with", *filename, "filename")
	ctx := context.Background()
	connStr := os.Getenv("CONNECTION_STRING")
	if connStr == "" {
		connStr = "postgresql://postgres:password@localhost:5432/homework"
		log.Println("Using default connection string (localhost)")
	}
	workQueue := make(chan *HostFilter, *workers*2)
	var waitGroup sync.WaitGroup

	toolOptions := ToolOptions{
		WorkerCount:      *workers,
		ParamFilename:    *filename,
		ConnectionString: connStr,
		Context:          &ctx,
		WorkerWaitGroup:  &waitGroup,
		HostFilterQueue:  workQueue,
		StatsQueue:       make(chan *Statistic, 5),
	}

	return &toolOptions
}

func main() {
	toolOptions := loadToolOptions()

	var statsWait sync.WaitGroup
	go processStatistics(toolOptions, &statsWait)
	go dispatchWorkAcrossWorkers(toolOptions)
	fillWorkQueue(toolOptions)

	toolOptions.WorkerWaitGroup.Wait()
	close(toolOptions.StatsQueue)
	statsWait.Wait()
}

// Calculates and expels the statistics to stdout
func processStatistics(opts *ToolOptions, statsWait *sync.WaitGroup) {
	statsWait.Add(1)
	count := 0
	errors := 0
	var timings []float64
	for stat := range opts.StatsQueue {
		count++
		if stat.IsError {
			errors++
			continue
		}

		nanoseconds := float64(stat.QueryDuration) * 1e-6
		timings = append(timings, nanoseconds)
	}

	mean, _ := stats.Mean(timings)
	median, _ := stats.Median(timings)
	sum, _ := stats.Sum(timings)
	min, _ := stats.Min(timings)
	max, _ := stats.Max(timings)

	fmt.Printf(`Statistics
============
Workers: %d
Total queries: %d
Errors: %d
Total time: %fms
Minimum query time: %fms
Mean: %fms
Median: %fms
Maximum query time: %fms
`,
		opts.WorkerCount,
		count, errors, sum,
		min, mean, median, max)
	statsWait.Done()
}

// Determines the index of the worker that should process the
// given host filter.
func getWorkerIndex(opts *ToolOptions, hostFilter *HostFilter) int {
	h := fnv.New32a()
	h.Write([]byte(hostFilter.Host))
	return int(h.Sum32()) % opts.WorkerCount
}

// Creates `workerCount` workers and dispatches tasks
// across them from the primary host filter queue (fan-out).
// Maintains consistency between worker and host names
// by hashing the host name.
func dispatchWorkAcrossWorkers(opts *ToolOptions) {
	var workers []*WorkerDef
	for i := 0; i < opts.WorkerCount; i++ {
		workers = append(workers, &WorkerDef{
			Id:              i + 1,
			HostFilterQueue: make(chan *HostFilter)})
	}

	for _, worker := range workers {
		opts.WorkerWaitGroup.Add(1)
		go runWorker(opts, worker)
	}

	go func() {
		for filter := range opts.HostFilterQueue {
			workerIndex := getWorkerIndex(opts, filter)
			workers[workerIndex].HostFilterQueue <- filter
		}

		for _, worker := range workers {
			close(worker.HostFilterQueue)
		}
	}()
}
