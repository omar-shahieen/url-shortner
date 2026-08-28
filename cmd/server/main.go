package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/repository/sqlite"
	"github.com/omar-shahieen/url-shortner/internal/routers"
	"github.com/omar-shahieen/url-shortner/internal/service"
	_ "modernc.org/sqlite"
)

func main() {
	database, err := sql.Open("sqlite", "url-shortener.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	repository := sqlite.New(database)
	if err := repository.Initialize(context.Background()); err != nil {
		log.Fatal(err)
	}

	shortener := service.New(repository)

	log.Println("URL shortener listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", routers.New(handler.New(shortener), database)); err != nil {
		log.Fatal(err)
	}
}
