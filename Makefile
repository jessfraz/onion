.PHONY: build dbuild drun dtor dtest-build dtest fmt lint shell test validate vet

# env vars passed through directly to test build scripts
DOCKER_ENVS := \
	-e DOCKER_GRAPHDRIVER \
	-e DOCKER_STORAGE_OPTS \

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := onion-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
DOCKER_RUN := docker run --rm -it --privileged $(DOCKER_ENVS) "$(DOCKER_IMAGE)"

all: build

build:
	go build ./...

dbuild:
	@docker build --rm --force-rm -t jess/onion .

drun:
	@docker run -d \
		--name onion \
		--cap-add NET_ADMIN \
		--net host \
		-v /run/docker/plugins:/run/docker/plugins \
		-v /var/run/docker.sock:/var/run/docker.sock \
		jess/onion -d

dtor:
	@docker run -d \
		-p 9050:9050 \
		-p 9040:9040 \
		-p 5353:5353 \
		--name tor-router \
		jess/tor-router

dtest-build:
	docker build --rm --force-rm -t "$(DOCKER_IMAGE)" -f hack/Dockerfile .

dtest: dtest-build
	$(DOCKER_RUN)

fmt:
	@gofmt -s -l .

lint:
	@golint ./...

shell: test-build
	$(DOCKER_RUN) bash

test: validate
	@go test -v ./...

validate: fmt lint vet

vet:
	@go vet ./...
