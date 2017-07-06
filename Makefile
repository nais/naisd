SHELL   := bash
VERSION := $(shell /bin/date +%Y%m%d%H%M%S)-$(shell git rev-parse --short HEAD)
NAME    := navikt/naisd
IMAGE   := "docker.adeo.no:5000/"${NAME}:${VERSION}
LATEST   := ${NAME}:latest

dockerhub-release: linux docker-build push-dockerhub
minikube: linux docker-minikube-build deploy

test:
	go test $(shell glide novendor) --logtostderr=true

build:
	go build -o api

linux:
	GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags '-s' -o naisd

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${IMAGE} -t ${NAME} -t ${LATEST} -f Dockerfile .

docker-build:
	docker image build -t ${IMAGE} -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${IMAGE}

deploy:
	helm upgrade -i naisd helm/naisd --set image.tag=${VERSION}