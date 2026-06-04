include .env

build:
	go build -o bin/fishSim ./cmd/api

run: build
	./bin/fishSim