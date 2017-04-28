SHELL := bash

test:
	go test $(shell glide novendor)

build:
	go build -o api

linux:
	GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags '-s' -o api