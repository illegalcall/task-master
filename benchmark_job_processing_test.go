// benchmark_test.go
package main

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Job represents a simplified job structure.
type Job struct {
	ID   int
	Name string
}

// processJobLogicForBenchmark simulates the actual job processing logic.
// In production, processJobLogic may sleep for 10s to simulate heavy work,
// but here we use a 1ms sleep to make the benchmark run quickly.
func processJobLogicForBenchmark(job Job) error {
	// Simulate processing time.
	time.Sleep(1 * time.Millisecond)
	// Optionally simulate an error (e.g., for jobs with certain IDs)
	// if job.ID%1000 == 0 {
	//     return fmt.Errorf("simulated error for job %d", job.ID)
	// }
	return nil
}

// BenchmarkJobProcessing measures the throughput of processing a large number
// of jobs concurrently using a worker pool.
func BenchmarkJobProcessing(b *testing.B) {
	// Report memory allocations (optional).
	b.ReportAllocs()

	// Create a channel to queue jobs.
	jobs := make(chan Job, b.N)

	// Define the number of concurrent workers.
	numWorkers := 50
	var wg sync.WaitGroup

	// Create an error channel to capture errors from workers.
	errCh := make(chan error, numWorkers)

	// Start the worker pool.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				if err := processJobLogicForBenchmark(job); err != nil {
					// Send the error to the error channel and return.
					errCh <- fmt.Errorf("Worker %d: job %d failed: %v", workerID, job.ID, err)
					return
				}
			}
		}(i)
	}

	// Reset the benchmark timer after the worker pool has been set up.
	b.ResetTimer()

	// Enqueue b.N jobs into the channel.
	for i := 0; i < b.N; i++ {
		jobs <- Job{
			ID:   i,
			Name: fmt.Sprintf("Job-%d", i),
		}
	}
	close(jobs)

	// Wait until all workers have finished processing.
	wg.Wait()
	close(errCh)

	// Check if any errors occurred.
	for err := range errCh {
		if err != nil {
			b.Fatal(err)
		}
	}
}
