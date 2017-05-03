SHELL   := bash
VERSION := $(shell /bin/date +%Y%m%d%H%M%S)-$(shell git rev-parse --short HEAD)
NAME    := navikt/naisd
IMAGE   := ${NAME}:${VERSION}

container: linux docker
minikube: container deploy

test:
	go test $(shell glide novendor)

build:
	go build -o api

linux:
	GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags '-s' -o naisd

docker:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${IMAGE} -t ${NAME} -f Dockerfile .

deploy:
	helm upgrade -i naisd helm/naisd --set image.tag=${VERSION}