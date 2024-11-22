package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	dataStore = make(map[string][]string)
	waiters   = make(map[string][]chan string)
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

	val, ok := waiters[key]
	if ok && len(val) > 0 {
		ch := val[0]
		waiters[key] = val[1:]
		ch <- value
		return
	}

	dataStore[key] = append(dataStore[key], value)

	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, dataStore[key])
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

	value, ok := dataStore[key]
	if ok && len(value) > 0 {
		val := value[0]
		dataStore[key] = value[1:]
		fmt.Fprint(w, val)
		return
	}

	if timeout > 0 {
		ch := make(chan string)

		waiters[key] = append(waiters[key], ch)

		select {
		case val := <-ch:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, val)
		case <-time.After(timeout):
			w.WriteHeader(http.StatusNotFound)
		}

		for i, waiter := range waiters[key] {
			if waiter == ch {
				waiters[key] = append(waiters[key][:i], waiters[key][i+1:]...)
				break
			}
		}
		return
	}
	http.Error(w, "", http.StatusNotFound)
}
