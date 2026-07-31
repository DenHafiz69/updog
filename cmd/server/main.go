package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type targetUrl struct {
	URL string `json:"url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func handlingError(w http.ResponseWriter, status int, errorMessage string) {
	respBody := errorResponse{
		Error: errorMessage,
	}
	writeJSON(w, status, respBody)
}

type pingStatus struct {
	Duration time.Duration `json:"duration"`
}

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /url", func(w http.ResponseWriter, r *http.Request) {

		// Ping url, return a 200 OK status
		newUrl := targetUrl{}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&newUrl)
		if err != nil {
			handlingError(w, 400, "invalid url")
			return
		}

		start := time.Now()

		_, err = http.Get(newUrl.URL)
		if err != nil {
			handlingError(w, 400, "can't get url")
			return
		}

		duration := time.Since(start)

		fmt.Printf("duration: %s\n", duration)
		urlStatus := pingStatus{
			Duration: duration,
		}

		writeJSON(w, 200, urlStatus)

	})

	fmt.Println("Server listening on :8080")

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}

}
