// Package handlers implements HTTP handler functions for the URL shortener service.
// It provides endpoints for creating, retrieving, and deleting shortened URLs.
package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

// BatchDeleteHandler handles the deletion of multiple shortened URLs for an authenticated user.
// It expects a DELETE request with a JSON array of short URL IDs.
// Upon successful deletion, it responds with a 202 Accepted status and processes the deletion asynchronously.
//
// Possible error codes in response:
// - 400 (Bad Request) if the request body is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 500 (Internal Server Error) if the server fails.
func BatchDeleteHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Authenticate the user and retrieve the user ID.
		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		var ids []string
		// Decode the JSON request body into a slice of short URL IDs.
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&ids); err != nil || len(ids) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Prepend the base URL to each ID to form the complete short URLs.
		for i, id := range ids {
			ids[i] = cfg.BaseURL + "/" + id
		}

		// Respond with a 202 Accepted status indicating that the deletion is being processed.
		w.WriteHeader(http.StatusAccepted)
		// Process the batch deletion asynchronously.
		go processBatchDelete(store, ids, userID)
	}
}

// processBatchDelete handles the asynchronous processing of batch deletions.
// It utilizes goroutines and channels to efficiently delete multiple URLs concurrently.
func processBatchDelete(store URLStore, ids []string, userID string) {
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Initialize the generator to emit URL IDs.
	inputCh := generator(doneCh, ids)
	// Fan out the deletion tasks across multiple workers.
	channels := fanOut(store, doneCh, inputCh, userID)
	// Fan in the results from all workers.
	resultCh := fanIn(doneCh, channels...)

	// Iterate over the results and log any errors encountered during deletion.
	for err := range resultCh {
		if err != nil {
			logging.Sugar.Errorw("Failed to delete URL", err)
		}
	}
}

// generator creates a channel that emits URL IDs for deletion.
// It reads from the input slice and sends each ID to the returned channel.
// The generator stops emitting if the doneCh is closed.
func generator(doneCh chan struct{}, input []string) chan string {
	inputCh := make(chan string)

	go func() {
		defer close(inputCh)

		for _, data := range input {
			select {
			case <-doneCh:
				return
			case inputCh <- data:
			}
		}
	}()

	return inputCh
}

// fanOut starts multiple worker goroutines to process URL deletions concurrently.
// It returns a slice of channels where each channel receives error results from a worker.
func fanOut(store URLStore, doneCh chan struct{}, inputCh chan string, userID string) []chan error {
	// Define the number of concurrent workers.
	numWorkers := 5
	// Initialize a slice to hold the result channels from each worker.
	channels := make([]chan error, numWorkers)

	// Start each worker goroutine.
	for i := 0; i < numWorkers; i++ {
		// Obtain a result channel from the deleteURL worker.
		addResultCh := deleteURL(store, doneCh, inputCh, userID)
		// Add the result channel to the channels slice.
		channels[i] = addResultCh
	}

	// Return the slice of result channels.
	return channels
}

// fanIn merges multiple error channels into a single channel.
// It listens to all provided resultChs and sends any received errors to the finalCh.
// Once all resultChs are closed, it closes the finalCh.
func fanIn(doneCh chan struct{}, resultChs ...chan error) chan error {
	// Initialize the final output channel.
	finalCh := make(chan error)

	// Use a WaitGroup to wait for all goroutines to finish.
	var wg sync.WaitGroup

	// Iterate over each result channel.
	for _, ch := range resultChs {
		// Do this because it's not allowed to send ch to the goroutine.
		chClosure := ch

		// Increment the WaitGroup counter.
		wg.Add(1)

		// Start a goroutine for each result channel.
		go func() {
			defer wg.Done()

			// Listen for errors from the result channel.
			for err := range chClosure {
				select {
				// Leave goroutine if the channel is closed.
				case <-doneCh:
					return
				// Send data to the final channel if it's not closed.
				case finalCh <- err:
				}
			}
		}()
	}

	// Start a goroutine to close the finalCh once all workers are done.
	go func() {
		// Wait for all goroutines.
		wg.Wait()
		// Close the final channel when all goroutines are done.
		close(finalCh)
	}()

	// Return the merged error channel.
	return finalCh
}

// deleteURL processes the deletion of a single URL.
// It reads URL IDs from the inputCh and attempts to delete them using the storage.
// Any errors encountered are sent to the resultCh.
func deleteURL(store URLStore, doneCh chan struct{}, inputCh chan string, userID string) chan error {
	resultCh := make(chan error)

	go func() {
		defer close(resultCh)
		for id := range inputCh {
			err := store.BatchUpdateDeleteFlag(id, userID)
			select {
			case <-doneCh:
				return
			case resultCh <- err:
			}
		}
	}()

	return resultCh
}
