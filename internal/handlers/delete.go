package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

func BatchDeleteHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		var ids []string
		err = json.Unmarshal(body, &ids)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		for i, id := range ids {
			ids[i] = fmt.Sprintf("%s/%s", cfg.BaseURL, id)
		}

		w.WriteHeader(http.StatusAccepted)
		go processBatchDelete(store, ids, userID)
	}
}

func processBatchDelete(store URLStore, ids []string, userID string) {
	doneCh := make(chan struct{})
	defer close(doneCh)

	inputCh := generator(doneCh, ids)
	channels := fanOut(store, doneCh, inputCh, userID)
	resultCh := fanIn(doneCh, channels...)

	for err := range resultCh {
		if err != nil {
			logging.Sugar.Errorw("Failed to delete URL", err)
		}
	}
}

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

func fanOut(store URLStore, doneCh chan struct{}, inputCh chan string, userID string) []chan error {
	// количество горутин add
	numWorkers := 5
	// каналы, в которые отправляются результаты
	channels := make([]chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		// получаем канал из горутины add
		addResultCh := deleteURL(store, doneCh, inputCh, userID)
		// отправляем его в слайс каналов
		channels[i] = addResultCh
	}

	// возвращаем слайс каналов
	return channels
}

// fanIn объединяет несколько каналов resultChs в один.
func fanIn(doneCh chan struct{}, resultChs ...chan error) chan error {
	// конечный выходной канал в который отправляем данные из всех каналов из слайса, назовём его результирующим
	finalCh := make(chan error)

	// понадобится для ожидания всех горутин
	var wg sync.WaitGroup

	// перебираем все входящие каналы
	for _, ch := range resultChs {
		// в горутину передавать переменную цикла нельзя, поэтому делаем так
		chClosure := ch

		// инкрементируем счётчик горутин, которые нужно подождать
		wg.Add(1)

		go func() {
			// откладываем сообщение о том, что горутина завершилась
			defer wg.Done()

			// получаем данные из канала
			for err := range chClosure {
				select {
				// выходим из горутины, если канал закрылся
				case <-doneCh:
					return
				// если не закрылся, отправляем данные в конечный выходной канал
				case finalCh <- err:
				}
			}
		}()
	}

	go func() {
		// ждём завершения всех горутин
		wg.Wait()
		// когда все горутины завершились, закрываем результирующий канал
		close(finalCh)
	}()

	// возвращаем результирующий канал
	return finalCh
}

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
