package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL     = "http://localhost:8080"
	numRequests = 1000
	concurrency = 50
)

type Stats struct {
	totalRequests      int64
	successRequests    int64
	failedRequests     int64
	totalDuration      time.Duration
	sumRequestDuration time.Duration
	minDuration        time.Duration
	maxDuration        time.Duration
	p95Duration        time.Duration
	p99Duration        time.Duration
}

func main() {
	fmt.Println("Starting load test...")
	fmt.Printf("Configuration: %d requests, %d concurrent workers\n\n", numRequests, concurrency)

	setupTestData()

	time.Sleep(1 * time.Second)

	tests := []struct {
		name string
		fn   func() time.Duration
	}{
		{"GET /statistics", testGetStatistics},
		{"POST /team/deactivateAll", testDeactivateTeam},
		{"GET /team/get", testGetTeam},
		{"POST /pullRequest/create", testCreatePR},
	}

	for _, test := range tests {
		fmt.Printf("Testing: %s\n", test.name)
		stats := runLoadTest(test.fn, numRequests, concurrency)
		printStats(stats)
		fmt.Println()
	}
}

func setupTestData() {
	ctx := context.Background()
	teams := []string{"team-load-1", "team-load-2", "team-load-3"}
	for _, teamName := range teams {
		data := map[string]interface{}{
			"team_name": teamName,
			"members": []map[string]interface{}{
				{"user_id": teamName + "-u1", "username": "User 1", "is_active": true},
				{"user_id": teamName + "-u2", "username": "User 2", "is_active": true},
				{"user_id": teamName + "-u3", "username": "User 3", "is_active": true},
			},
		}
		body, err := json.Marshal(data)
		if err != nil {
			fmt.Printf("Failed to marshal team data for %s: %v\n", teamName, err)
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/team/add", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("Failed to create request for team %s: %v\n", teamName, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Failed to add team %s: %v\n", teamName, err)
			continue
		}
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			fmt.Printf("Failed to read response body for team %s: %v\n", teamName, err)
		}
		err = resp.Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body for team %s: %v\n", teamName, err)
		}
	}
}

func runLoadTest(fn func() time.Duration, total, workers int) Stats {
	stats := Stats{
		minDuration: time.Hour,
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	durations := make(chan time.Duration, total)
	var successDurations []time.Duration
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			duration := fn()
			durations <- duration
		}()
	}

	go func() {
		wg.Wait()
		close(durations)
	}()

	for d := range durations {
		atomic.AddInt64(&stats.totalRequests, 1)
		if d >= 0 {
			atomic.AddInt64(&stats.successRequests, 1)
			mu.Lock()
			successDurations = append(successDurations, d)
			stats.sumRequestDuration += d
			if d < stats.minDuration {
				stats.minDuration = d
			}
			if d > stats.maxDuration {
				stats.maxDuration = d
			}
			mu.Unlock()
		} else {
			atomic.AddInt64(&stats.failedRequests, 1)
		}
	}

	stats.totalDuration = time.Since(start)

	if len(successDurations) > 0 {
		stats.p95Duration = calculatePercentile(successDurations, 95)
		stats.p99Duration = calculatePercentile(successDurations, 99)
	}

	return stats
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func printStats(stats Stats) {
	avgDuration := time.Duration(0)
	if stats.successRequests > 0 {
		avgDuration = stats.sumRequestDuration / time.Duration(stats.successRequests)
	}

	throughput := float64(stats.successRequests) / stats.totalDuration.Seconds()

	fmt.Printf("Results:\n")
	fmt.Printf("  Total requests:    %d\n", stats.totalRequests)
	fmt.Printf("  Successful:        %d (%.2f%%)\n", stats.successRequests, float64(stats.successRequests)/float64(stats.totalRequests)*100)
	fmt.Printf("  Failed:            %d\n", stats.failedRequests)
	fmt.Printf("  Wall clock time:   %v\n", stats.totalDuration.Round(time.Millisecond))
	fmt.Printf("  Min response time: %v\n", stats.minDuration.Round(time.Microsecond))
	fmt.Printf("  Avg response time: %v\n", avgDuration.Round(time.Microsecond))
	fmt.Printf("  P95 response time: %v\n", stats.p95Duration.Round(time.Microsecond))
	fmt.Printf("  P99 response time: %v\n", stats.p99Duration.Round(time.Microsecond))
	fmt.Printf("  Max response time: %v\n", stats.maxDuration.Round(time.Microsecond))
	fmt.Printf("  Throughput:        %.2f req/sec\n", throughput)
}

func testGetStatistics() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/statistics", nil)
	if err != nil {
		return -1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		fmt.Printf("Warning: failed to read response body: %v\n", err)
	}

	if resp.StatusCode != http.StatusOK {
		return -1
	}

	return time.Since(start)
}

func testDeactivateTeam() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data := map[string]string{
		"team_name": "team-load-1",
	}
	body, err := json.Marshal(data)
	if err != nil {
		return -1
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/team/deactivateAll", bytes.NewReader(body))
	if err != nil {
		return -1
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		fmt.Printf("Warning: failed to read response body: %v\n", err)
	}

	return time.Since(start)
}

func testGetTeam() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/team/get?team_name=team-load-2", nil)
	if err != nil {
		return -1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		fmt.Printf("Warning: failed to read response body: %v\n", err)
	}

	if resp.StatusCode != http.StatusOK {
		return -1
	}

	return time.Since(start)
}

var prCounter int64

func testCreatePR() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id := atomic.AddInt64(&prCounter, 1)
	data := map[string]string{
		"pull_request_id":   fmt.Sprintf("pr-load-%d", id),
		"pull_request_name": fmt.Sprintf("Load test PR %d", id),
		"author_id":         "team-load-3-u1",
	}
	body, err := json.Marshal(data)
	if err != nil {
		return -1
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/pullRequest/create", bytes.NewReader(body))
	if err != nil {
		return -1
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		fmt.Printf("Warning: failed to read response body: %v\n", err)
	}

	return time.Since(start)
}
