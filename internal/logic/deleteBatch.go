package logic

import (
	"sync"

	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

// BatchDeleteAsync handles the deletion of multiple shortened URLs for an authenticated user.
func (s *ShortenerService) BatchDeleteAsync(userID string, ids []string) {
	// Prepend the base URL to each ID to form the complete short URLs.
	for i := range ids {
		ids[i] = s.Cfg.BaseURL + "/" + ids[i]
	}

	// Process the batch deletion asynchronously.
	go s.processBatchDelete(ids, userID)
}

// processBatchDelete handles the asynchronous processing of batch deletions.
// It utilizes goroutines and channels to efficiently delete multiple URLs concurrently.
func (s *ShortenerService) processBatchDelete(ids []string, userID string) {
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Initialize the generator to emit URL IDs.
	inputCh := s.generator(doneCh, ids)
	// Fan out the deletion tasks across multiple workers.
	channels := s.fanOut(doneCh, inputCh, userID)
	// Fan in the results from all workers.
	resultCh := s.fanIn(doneCh, channels...)

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
func (s *ShortenerService) generator(doneCh chan struct{}, input []string) chan string {
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
func (s *ShortenerService) fanOut(doneCh chan struct{}, inputCh chan string, userID string) []chan error {
	// Define the number of concurrent workers.
	numWorkers := 5
	// Initialize a slice to hold the result channels from each worker.
	channels := make([]chan error, numWorkers)

	// Start each worker goroutine.
	for i := 0; i < numWorkers; i++ {
		// Obtain a result channel from the deleteURL worker.
		addResultCh := s.deleteURL(doneCh, inputCh, userID)
		// Add the result channel to the channels slice.
		channels[i] = addResultCh
	}

	// Return the slice of result channels.
	return channels
}

// fanIn merges multiple error channels into a single channel.
// It listens to all provided resultChs and sends any received errors to the finalCh.
// Once all resultChs are closed, it closes the finalCh.
func (s *ShortenerService) fanIn(doneCh chan struct{}, resultChs ...chan error) chan error {
	// Initialize the final output channel.
	finalCh := make(chan error)

	// Use a WaitGroup to wait for all goroutines to finish.
	var wg sync.WaitGroup

	// Iterate over each result channel.
	for _, ch := range resultChs {
		// Do this because it's not allowed to send ch variable to the goroutine.
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
func (s *ShortenerService) deleteURL(doneCh chan struct{}, inputCh chan string, userID string) chan error {
	resultCh := make(chan error)

	go func() {
		defer close(resultCh)
		for id := range inputCh {
			err := s.Store.BatchUpdateDeleteFlag(id, userID)
			select {
			case <-doneCh:
				return
			case resultCh <- err:
			}
		}
	}()

	return resultCh
}
