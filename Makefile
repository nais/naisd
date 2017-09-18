SHELL   := bash
NAME    := navikt/naisd
LATEST  := ${NAME}:latest
GLIDE   := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd navikt/glide glide
GO_IMG  := golang:1.8
GO      := sudo docker run --rm -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd ${GO_IMG} go

dockerhub-release: install test linux bump tag docker-build push-dockerhub
minikube: linux docker-minikube-build helm-upgrade

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

build-cli:
    ${GO} build -o nais ./cli/

linux:
	sudo docker run --rm -e GOOS=linux -e CGO_ENABLED=0 -v ${PWD}:/go/src/github.com/nais/naisd -w /go/src/github.com/nais/naisd ${GO_IMG} go build -a -installsuffix cgo -ldflags '-s' -o naisd

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t ${NAME} -t ${LATEST} -f Dockerfile --no-cache .

docker-build:
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t naisd -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${NAME}:$(shell /bin/cat ./version)

helm-upgrade:
	helm delete naisd; helm upgrade -i naisd helm/naisd --set image.version=$(shell /bin/cat ./version)
