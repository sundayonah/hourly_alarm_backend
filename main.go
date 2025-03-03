package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/sundayonah/hourly_alarm/routes"
)

func main() {

	// Set up API routes and middleware
	routes.Routes()

	port := 8080
	log.Printf("Starting server on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
