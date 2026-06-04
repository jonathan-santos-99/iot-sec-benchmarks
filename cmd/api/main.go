package main

import (
	"log"

	auth "fishSim/internal"
	"fishSim/views"
)

type application struct {
	authService *auth.Service
	renderer    *views.Renderer
	sessions    map[string]string
}

func main() {
	authService := auth.NewService()
	renderer, err := views.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}

	app := application{
		authService: authService,
		renderer:    renderer,
		sessions:    make(map[string]string),
	}

	app.serve()
}
