package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect" // Added for DeepEqual comparison
	"sync"
	"sync/atomic" // Correct import for atomic operations
	"time"
)

const (
	minLedgerIndex = 32750
	maxLedgerIndex = 99000000
	numSamples     = 50000
	maxConcurrency = 1 // Limit concurrent requests per batch (Set to 1 as requested by user context)
	maxRetries     = 5 // Max retries on errors

	endpoint1 = "http://35.89.58.127:51233" // TODO: make sure to change this to whichever IP address we want to compare to Clio Mainnet
	endpoint2 = "https://s1.ripple.com:51233"
)

// Define the structure for the JSON request body
type LedgerDataRequest struct {
	Method string        `json:"method"`
	Params []LedgerParam `json:"params"`
}

type LedgerParam struct {
	LedgerIndex int  `json:"ledger_index"`
	Binary      bool `json:"binary"`
	Limit       int
}

// Define structure to check for "tooBusy" error response
type ErrorResponse struct {
	Error        string `json:"error"` // Check top-level error field
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Status       string `json:"status"`
	Type         string `json:"type"`
}

// Function to make the HTTP POST request and return the response body with retry logic
func makeRequest(url string, ledgerIndex int) ([]byte, error) {
	requestBody := LedgerDataRequest{
		Method: "ledger_data",
		Params: []LedgerParam{
			{LedgerIndex: ledgerIndex, Binary: true, Limit: 10},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling JSON for %d: %w", ledgerIndex, err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a new buffer for each attempt
		reqBody := bytes.NewBuffer(jsonData)
		// Use a client with a timeout
		client := &http.Client{Timeout: 100 * time.Second} // Add a reasonable timeout
		resp, err := client.Post(url, "application/json", reqBody)

		if err != nil {
			lastErr = fmt.Errorf("attempt %d: error making request to %s for %d: %w", attempt, url, ledgerIndex, err)
			log.Printf("%v - Retrying...\n", lastErr)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond) // Exponential backoff
			continue                                                    // Retry request
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() // Close body immediately after reading

		if readErr != nil {
			lastErr = fmt.Errorf("attempt %d: error reading response body from %s for %d: %w", attempt, url, ledgerIndex, readErr)
			log.Printf("%v - Retrying...\n", lastErr)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond) // Exponential backoff
			continue                                                    // Retry request reading
		}

		if resp.StatusCode == http.StatusOK {
			return bodyBytes, nil // Success!
		}

		// Check if the non-OK response contains the "tooBusy" error
		var errorResp ErrorResponse
		jsonErr := json.Unmarshal(bodyBytes, &errorResp)

		if jsonErr == nil && errorResp.Error == "tooBusy" {
			lastErr = fmt.Errorf("attempt %d: server busy error from %s for %d: %s", attempt, url, ledgerIndex, string(bodyBytes))
			log.Printf("%v - Retrying...\n", lastErr)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Longer backoff for busy
			continue                                                    // Retry request
		}

		// If it's a different error or not the "tooBusy" error, don't retry, return the error immediately
		lastErr = fmt.Errorf("bad status code from %s for %d: %d, body: %s", url, ledgerIndex, resp.StatusCode, string(bodyBytes))
		// Don't return nil here, return the bodyBytes for potential debugging in main
		return bodyBytes, lastErr // Return final error after bad status, include bodyBytes

	} // End of retry loop

	// If all retries failed, return the last error encountered
	return nil, fmt.Errorf("all %d retry attempts failed for %s ledger %d. Last error: %w", maxRetries, url, ledgerIndex, lastErr)
}

func main() {
	// Seed the random number generator
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	// Generate unique random ledger indices
	ledgerIndicesMap := make(map[int]struct{})
	fmt.Printf("Generating %d unique random ledger indices...\n", numSamples)
	for len(ledgerIndicesMap) < numSamples {
		index := rng.Intn(maxLedgerIndex-minLedgerIndex+1) + minLedgerIndex
		ledgerIndicesMap[index] = struct{}{}
	}
	fmt.Println("Done generating indices.")

	// Convert map keys to slice for easier batching
	ledgerIndices := make([]int, 0, len(ledgerIndicesMap))
	for k := range ledgerIndicesMap {
		ledgerIndices = append(ledgerIndices, k)
	}

	// results channel removed
	// results := make(chan string, numSamples)

	// --- Use atomic counters for thread-safe updates ---
	var mismatches atomic.Int64
	var errorCount atomic.Int64
	var processedCount atomic.Int64
	// ---------------------------------------------------

	fmt.Printf("Starting comparisons for %d ledgers with batch concurrency %d...\n", numSamples, maxConcurrency)

	// --- Process indices in batches ---
	for i := 0; i < len(ledgerIndices); i += maxConcurrency {
		end := i + maxConcurrency
		if end > len(ledgerIndices) {
			end = len(ledgerIndices)
		}
		batchIndices := ledgerIndices[i:end]

		var wg sync.WaitGroup // Use a WaitGroup for each batch

		// Removed batch processing message as results are printed immediately
		// fmt.Printf("Processing batch %d-%d...\n", i+1, end)

		for _, index := range batchIndices {
			wg.Add(1)

			go func(idx int) {
				defer wg.Done()

				// Make requests concurrently within the batch
				resp1Chan := make(chan []byte)
				err1Chan := make(chan error)
				resp2Chan := make(chan []byte)
				err2Chan := make(chan error)

				go func() {
					resp, err := makeRequest(endpoint1, idx)
					// Always send something to the channel, even if error
					err1Chan <- err
					resp1Chan <- resp
				}()
				go func() {
					resp, err := makeRequest(endpoint2, idx)
					// Always send something to the channel, even if error
					err2Chan <- err
					resp2Chan <- resp

				}()

				// Wait for both responses/errors from this specific index
				err1 := <-err1Chan
				resp1 := <-resp1Chan
				err2 := <-err2Chan
				resp2 := <-resp2Chan

				// Handle errors first
				if err1 != nil {
					// Print error immediately
					fmt.Fprintf(os.Stderr, "Ledger %d: ERROR requesting from %s: %v\n", idx, endpoint1, err1)
					errorCount.Add(1) // Atomic increment
					return            // Stop processing this index on error
				}
				if err2 != nil {
					// Print error immediately
					fmt.Fprintf(os.Stderr, "Ledger %d: ERROR requesting from %s: %v\n", idx, endpoint2, err2)
					errorCount.Add(1) // Atomic increment
					return            // Stop processing this index on error
				}

				// --- START MODIFIED COMPARISON LOGIC ---

				// Unmarshal both responses into maps
				var data1, data2 map[string]interface{}

				err1 = json.Unmarshal(resp1, &data1)
				if err1 != nil {
					fmt.Fprintf(os.Stderr, "Ledger %d: ERROR unmarshalling JSON from %s: %v\nBody: %s\n", idx, endpoint1, err1, string(resp1))
					errorCount.Add(1)
					return // Can't compare if unmarshalling fails
				}

				err2 = json.Unmarshal(resp2, &data2)
				if err2 != nil {
					fmt.Fprintf(os.Stderr, "Ledger %d: ERROR unmarshalling JSON from %s: %v\nBody: %s\n", idx, endpoint2, err2, string(resp2))
					errorCount.Add(1)
					return // Can't compare if unmarshalling fails
				}

				// Remove the "warning" key from the top level of both maps
				// This is safe even if the key doesn't exist
				delete(data1, "warning")
				delete(data2, "warning")

				// Compare the modified maps
				if reflect.DeepEqual(data1, data2) {
					// Print OK immediately
					fmt.Printf("Ledger %d: OK\n", idx)
				} else {
					// Print MISMATCH immediately
					fmt.Fprintf(os.Stderr, "Ledger %d: MISMATCH\n", idx)
					mismatches.Add(1) // Atomic increment
					// Optional: Log the original differing responses
					// log.Printf("Ledger %d MISMATCH:\nResp1: %s\nResp2: %s\n", idx, string(resp1), string(resp2))
				}
				// --- END MODIFIED COMPARISON LOGIC ---

				processedCount.Add(1) // Increment processed count only on success/mismatch
			}(index)
		}

		// Wait for the current batch of goroutines to finish before starting the next batch
		wg.Wait()
		// Removed batch finished message as results are printed immediately
		// fmt.Printf("Batch %d-%d finished. Total processed so far: %d\n", i+1, end, processedCount.Load())

	} // End of batch loop
	// --------------------------

	// close(results) // No longer needed

	fmt.Println("\n--- Comparison Complete ---")

	// Print summary
	finalMismatches := mismatches.Load()
	finalErrors := errorCount.Load()
	finalProcessed := processedCount.Load() // Use atomic load for final count

	// Section to print errors/mismatches from results channel removed

	fmt.Printf("\nTotal Ledgers Checked Attempted: %d\n", numSamples)
	fmt.Printf("Total Ledgers Successfully Processed (OK or Mismatch): %d\n", finalProcessed)
	fmt.Printf("Mismatches Found: %d\n", finalMismatches)
	fmt.Printf("Errors Encountered (Requests failed after retries or JSON parse failed): %d\n", finalErrors)
}
