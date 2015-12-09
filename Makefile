.PHONY: build dbuild drun dtor dtest-build dtest fmt lint shell test validate vet

# env vars passed through directly to test build scripts
DOCKER_ENVS := \
	-e DOCKER_GRAPHDRIVER \
	-e DOCKER_STORAGE_OPTS \

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := onion-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))

# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif

DOCKER_RUN := docker run --rm -i $(DOCKER_FLAGS) --privileged $(DOCKER_ENVS) "$(DOCKER_IMAGE)"

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
		-p 5353:5353/udp \
		--name tor-router \
		jess/tor-router

dtest-build:
	docker build --rm --force-rm -t "$(DOCKER_IMAGE)" -f Dockerfile.test .

dtest: dtest-build
	$(DOCKER_RUN)

fmt:
	@gofmt -s -l . | grep -v vendor | tee /dev/stderr

lint:
	@golint ./... | grep -v vendor | tee /dev/stderr

shell: dtest-build
	$(DOCKER_RUN) bash

test: validate
	go test -v $(shell go list ./... | grep -v /vendor/src)

validate: fmt lint

vet:
	go vet $(shell go list ./... | grep -v vendor)
