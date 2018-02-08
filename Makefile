SHELL     := bash
NAME      := navikt/naisd
LATEST    := ${NAME}:latest
GLIDE_IMG := navikt/glide:2.0.0
GLIDE     := docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd ${GLIDE_IMG} glide
GO_IMG    := golang:1.9
GO        := docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd ${GO_IMG} go
LDFLAGS   := -X github.com/nais/naisd/api/version.Revision=$(shell git rev-parse --short HEAD) -X github.com/nais/naisd/api/version.Version=$(shell /bin/cat ./version)

.PHONY: dockerhub-release install test linux bump tag cli cli-dist build docker-build push-dockerhub docker-minikube-build helm-upgrade

dockerhub-release: install test linux bump tag docker-build push-dockerhub
minikube: linux docker-minikube-build helm-upgrade

bump:
	/bin/bash bump.sh

tag:
	git tag -a $(shell /bin/cat ./version) -m "auto-tag from Makefile [skip ci]" && git push --tags

install:
	${GLIDE} install --strip-vendor

test:
	${GO} test ./api/ ./cli

cli:
	${GO} build -ldflags='$(LDFLAGS)' -o nais ./cli


cli-dist:
	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/naisd \
		-w /go/src/github.com/nais/naisd \
		-e GOOS=linux \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o nais-linux-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	sudo xz nais-linux-amd64

	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/naisd \
		-w /go/src/github.com/nais/naisd \
		-e GOOS=darwin \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o nais-darwin-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	sudo xz nais-darwin-amd64

	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/naisd \
		-w /go/src/github.com/nais/naisd \
		-e GOOS=windows \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o nais-windows-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/nais.go
	zip -r nais-windows-amd64.zip nais-windows-amd64
	sudo rm nais-windows-amd64

build:
	${GO} build -o naisd

linux:
	docker run --rm \
		-e GOOS=linux \
		-e CGO_ENABLED=0 \
		-v ${PWD}:/go/src/github.com/nais/naisd \
		-w /go/src/github.com/nais/naisd ${GO_IMG} \
		go build -a -installsuffix cgo -ldflags '-s $(LDFLAGS)' -o naisd

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t ${NAME} -t ${LATEST} -f Dockerfile --no-cache .

docker-build:
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t naisd -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${NAME}:$(shell /bin/cat ./version)

helm-upgrade:
	helm delete naisd; helm upgrade -i naisd helm/naisd --set image.version=$(shell /bin/cat ./version)
