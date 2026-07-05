package main

import (
	"log"
	"net/http"
	"os"

	"snippetsync/backend/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := api.NewServer()
	log.Printf("SnippetSync API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
