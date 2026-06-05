package main

import (
	"flag"
	"log"

	"fishSim/internal/auth"
	"fishSim/internal/metrics"
	"fishSim/views"
)

type application struct {
	authService    *auth.Service
	metricsService *metrics.Service
	renderer       *views.Renderer
	sessions       map[string]string
}

func main() {
	pwfile := flag.String("pwfile", "data/pwfile", "The full path for users file")
	flag.Parse()

	authService := auth.NewService(*pwfile)
	metricsService := metrics.NewService()
	renderer, err := views.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}

	app := application{
		authService:    authService,
		metricsService: metricsService,
		renderer:       renderer,
		sessions:       make(map[string]string),
	}

	app.serve()
}
