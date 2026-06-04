package main

import (
	"net/http"
)

func (app *application) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleLoginPage)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle("/login", http.HandlerFunc(app.handleLogin))
	mux.Handle("/home", app.authMiddleware(http.HandlerFunc(app.handleHomePage)))

	return mux
}
