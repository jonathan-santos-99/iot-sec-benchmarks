package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type envelope map[string]interface{}

func (app *application) writeJSON(
	w http.ResponseWriter, status int, data envelope) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	js = append(js, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(js)
	if err != nil {
		return err
	}

	return nil
}

func (app *application) errorResponse(w http.ResponseWriter, status int, message any) {
	envlp := envelope{"error": message}
	err := app.writeJSON(w, status, envlp)

	if err != nil {
		log.Printf("Error: %s", err)
		w.WriteHeader(500)
	}
}
