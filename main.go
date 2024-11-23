package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	dataStore sync.Map
	waiters   sync.Map
)

func main() {
	port := "80"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			handlePut(w, r)
		case r.Method == http.MethodGet:
			handleGet(w, r)
		default:
			http.Error(w, "", http.StatusMethodNotAllowed)
		}
	})

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	value := r.URL.Query().Get("v")

	if key == "" || value == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	val, _ := waiters.LoadOrStore(key, []chan string{})
	waitersSlice := val.([]chan string)

	if len(waitersSlice) > 0 {
		ch := waitersSlice[0]
		waiters.Store(key, waitersSlice[1:])
		ch <- value
		return
	}

	val, _ = dataStore.LoadOrStore(key, []string{})
	dataStore.Store(key, append(val.([]string), value))

	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, val.([]string))
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	timeoutStr := r.URL.Query().Get("timeout")
	var timeout time.Duration

	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	val, _ := dataStore.LoadOrStore(key, []string{})
	value := val.([]string)

	if len(value) > 0 {
		result := value[0]
		dataStore.Store(key, value[1:])
		fmt.Fprint(w, result)
		return
	}

	if timeout > 0 {
		ch := make(chan string)

		val, _ := waiters.LoadOrStore(key, []chan string{})
		waitersSlice := val.([]chan string)
		waiters.Store(key, append(waitersSlice, ch))

		select {
		case result := <-ch:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, result)
		case <-time.After(timeout):
			w.WriteHeader(http.StatusNotFound)
		}

		val, _ = waiters.LoadOrStore(key, []chan string{})
		waitersSlice = val.([]chan string)

		for i, waiter := range waitersSlice {
			if waiter == ch {
				waitersSlice = append(waitersSlice[:i], waitersSlice[i+1:]...)
				break
			}
		}
		waiters.Store(key, waitersSlice)
		return
	}

	http.Error(w, "", http.StatusNotFound)
}
