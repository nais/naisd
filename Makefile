SHELL   := bash
VERSION := $(shell /bin/date +%Y%m%d%H%M%S)-$(shell git rev-parse --short HEAD)
NAME    := navikt/nais-api
IMAGE   := ${NAME}:${VERSION}

container: linux docker
minikube: container deploy

test:
	go test $(shell glide novendor)

build:
	go build -o api

linux:
	GOOS=linux CGO_ENABLED=0 go build -v -x -a -installsuffix cgo -ldflags '-s' -o api

docker:
	@eval $$(minikube docker-env) ;\ # lets us use local images
	docker image build -t ${IMAGE} -t ${NAME} -f Dockerfile .

deploy:
	helm upgrade -i nais-api helm/nais-api --set image.tag=${VERSION}