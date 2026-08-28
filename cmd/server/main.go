package main

import (
	"log"
	"net/http"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/repository/inmemory"
	"github.com/omar-shahieen/url-shortner/internal/service"
)

func main() {
	repository := inmemory.New()
	shortener := service.New(repository)

	log.Println("URL shortener listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", handler.New(shortener)); err != nil {
		log.Fatal(err)
	}
}
