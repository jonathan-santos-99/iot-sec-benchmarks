package main

import (
	"log"
	"net/http"
)

func (app *application) serve() error {
	addr := ":8080"
	log.Printf("servidor disponível em http://localhost%s", addr)
	mux := app.mux()

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}

	return nil
}
