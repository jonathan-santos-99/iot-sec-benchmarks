package main

import (
	"flag"
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
	pwfile := flag.String("pwfile", "data/pwfile", "The full path for users file")
	flag.Parse()

	authService := auth.NewService(*pwfile)
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
