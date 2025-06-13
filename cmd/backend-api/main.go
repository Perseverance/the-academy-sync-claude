package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Backend API Service is running!")
	})

	log.Println("Backend API starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}