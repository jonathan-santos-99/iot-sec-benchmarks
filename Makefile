include .env

build:
	go build -o bin/fishSim ./cmd/api

run: build
	./bin/fishSim -pwfile data/pwfile

add_user: data/add_user.py
	data/add_user.py data/pwfile