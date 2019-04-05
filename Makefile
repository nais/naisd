export GO111MODULE=on
SHELL   := bash
NAME    := navikt/naisd
LATEST  := ${NAME}:latest
LDFLAGS := -X github.com/nais/naisd/api/version.Revision=$(shell git rev-parse --short HEAD) -X github.com/nais/naisd/api/version.Version=$(shell /bin/cat ./version)

.PHONY: dockerhub-release install test linux bump tag cli cli-dist build docker-build push-dockerhub docker-minikube-build helm-upgrade

all: install test linux

dockerhub-release: install test linux bump tag docker-build push-dockerhub

bump:
	/bin/bash bump.sh

tag:
	git tag -a $(shell /bin/cat ./version) -m "auto-tag from Makefile [skip ci]" && git push --tags

install:
	go get

test:
	go test ./... --coverprofile=cover.out

linux-cli:
	CGO_ENABLED=0 \
	GOOS=linux \
	go build -a -installsuffix cgo -o nais cli/nais.go

cli:
	go build -ldflags='$(LDFLAGS)' -o nais ./cli


cli-dist:
	GOOS=linux \
	GOARCH=amd64 \
	go build -o nais-linux-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	sudo xz nais-linux-amd64

	GOOS=darwin \
	GOARCH=amd64 \
	go build -o nais-darwin-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	sudo xz nais-darwin-amd64

	GOOS=windows \
	GOARCH=amd64 \
	go build -o nais-windows-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	zip -r nais-windows-amd64.zip nais-windows-amd64
	sudo rm nais-windows-amd64

build:
	go build -o naisd

linux:
	GOOS=linux \
	CGO_ENABLED=0 \
	go build -a -installsuffix cgo -ldflags '-s $(LDFLAGS)' -o naisd

docker-build:
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t naisd -t ${NAME} -t ${LATEST} -f Dockerfile .
	docker image build -t navikt/nais:$(shell /bin/cat ./version) -t navikt/nais:latest  -f Dockerfile.cli .

push-dockerhub:
	docker image push ${NAME}:$(shell /bin/cat ./version)
	docker image push navikt/nais:$(shell /bin/cat ./version)
	docker image push navikt/nais:latest

helm-upgrade:
	helm delete naisd; helm upgrade -i naisd helm/naisd --set image.version=$(shell /bin/cat ./version)
