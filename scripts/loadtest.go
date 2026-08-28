package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Base URL of the shortener")
	concurrency := flag.Int("c", 50, "Number of concurrent workers")
	duration := flag.Duration("d", 10*time.Second, "Duration of the test")
	verbose := flag.Bool("v", false, "Enable verbose logging for each request")
	flag.Parse()

	log.Printf("Starting load test on %s with %d workers for %v", *baseURL, *concurrency, *duration)

	// 1. First, create a short URL to test against.
	// We only do this once because the POST endpoint has a strict rate limit (10 req/s).
	code := setupShortURL(*baseURL)
	targetURL := fmt.Sprintf("%s/api/stats/%s", *baseURL, code)
	log.Printf("Created short URL '%s'. Hammering stats endpoint: %s\n\n", code, targetURL)

	// 2. Start the load test
	var (
		reqs       atomic.Uint64
		successes  atomic.Uint64
		failures   atomic.Uint64
		totalTime  atomic.Int64 // in microseconds
	)

	start := time.Now()
	deadline := start.Add(*duration)

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: 2 * time.Second,
				Transport: &http.Transport{
					MaxIdleConnsPerHost: 100, // optimize for high throughput to a single host
				},
			}

			for time.Now().Before(deadline) {
				reqStart := time.Now()
				reqs.Add(1)

				resp, err := client.Get(targetURL)
				
				latency := time.Since(reqStart)
				
				if err != nil {
					failures.Add(1)
					if *verbose {
						log.Printf("[Worker %d] GET %s -> ERROR: %v (latency: %v)\n", workerID, targetURL, err, latency)
					}
					continue
				}
				
				// Read and discard body so connection can be reused
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successes.Add(1)
				} else {
					failures.Add(1)
				}
				
				if *verbose {
					log.Printf("[Worker %d] GET %s -> Status: %d (latency: %v)\n", workerID, targetURL, resp.StatusCode, latency)
				}

				totalTime.Add(latency.Microseconds())
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// 3. Print results
	totalReqs := reqs.Load()
	avgLatency := time.Duration(totalTime.Load()/int64(totalReqs)) * time.Microsecond
	rps := float64(totalReqs) / elapsed.Seconds()

	fmt.Println("--- Load Test Results ---")
	fmt.Printf("Total Requests:  %d\n", totalReqs)
	fmt.Printf("Successes:       %d\n", successes.Load())
	fmt.Printf("Failures:        %d\n", failures.Load())
	fmt.Printf("Time Elapsed:    %v\n", elapsed)
	fmt.Printf("Reqs/Second:     %.2f\n", rps)
	fmt.Printf("Avg Latency:     %v\n", avgLatency)
}

func setupShortURL(baseURL string) string {
	payload := map[string]string{
		"originalUrl": "https://go.dev/doc/",
		"customAlias": fmt.Sprintf("loadtest-%d", time.Now().UnixNano()),
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(baseURL+"/api/shorten", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("Failed to create test URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		log.Fatalf("Failed to create test URL, status %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	return result.Code
}
