SHELL   := bash
VERSION := $(shell /bin/date +%Y%m%d%H%M%S)-$(shell git rev-parse --short HEAD)
NAME    := navikt/naisd
IMAGE   := ${NAME}:${VERSION}
LATEST  := ${NAME}:latest
GLIDE   := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd navikt/glide glide
GO      := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd golang:1.8 go
DEBUG   := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd golang:1.8 pwd; ls

travis-debug: debug
dockerhub-release: install test linux docker-build push-dockerhub
minikube: linux docker-minikube-build deploy

install:
	${GLIDE} install --strip-vendor

test:
	${GO} test ./api/

debug:
	${DEBUG}

build:
	${GO} build -o naisd

linux:
	GOOS=linux CGO_ENABLED=0 ${GO} build -a -installsuffix cgo -ldflags '-s' -o naisd

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${IMAGE} -t ${NAME} -t ${LATEST} -f Dockerfile .

docker-build:
	docker image build -t ${IMAGE} -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${IMAGE}

deploy:
	helm upgrade -i naisd helm/naisd --set image.tag=${VERSION}
