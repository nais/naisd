SHELL   := bash
NAME    := navikt/naisd
LATEST  := ${NAME}:latest
GLIDE   := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd navikt/glide glide
GO      := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd golang:1.8 go

dockerhub-release: install test linux bump tag docker-build push-dockerhub
minikube: linux docker-minikube-build deploy

bump:
	/bin/bash bump.sh 

tag:
	git tag -a $(shell /bin/cat ./version) -m "auto-tag from Makefile [skip ci]" && git push --tags

install:
	${GLIDE} install --strip-vendor

test:
	${GO} test ./api/

build:
	${GO} build -o naisd

linux:
	GOOS=linux CGO_ENABLED=0 ${GO} build -a -installsuffix cgo -ldflags '-s' -o naisd

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t ${NAME} -t ${LATEST} -f Dockerfile .

docker-build:
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${NAME}:$(shell /bin/cat ./version)

deploy:
	helm upgrade -i naisd helm/naisd --set image.tag=${LATEST}
