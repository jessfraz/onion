.PHONY: build clean dbuild drun dtor dtest-build dtest fmt lint shell test validate vet

# set the graph driver as the current graphdriver if not set
DOCKER_GRAPHDRIVER := $(if $(DOCKER_GRAPHDRIVER),$(DOCKER_GRAPHDRIVER),$(shell docker info | grep "Storage Driver" | sed 's/.*: //'))

# env vars passed through directly to test build scripts
DOCKER_ENVS := \
	-e DOCKER_GRAPHDRIVER=$(DOCKER_GRAPHDRIVER) \
	-e DOCKER_STORAGE_OPTS \

# to allow `make BIND_DIR=. shell` or `make BIND_DIR= test`
# (default to no bind mount if DOCKER_HOST is set)
# note: BINDDIR is supported for backwards-compatibility here
BIND_DIR := $(if $(BINDDIR),$(BINDDIR),$(if $(DOCKER_HOST),,logs))
DOCKER_MOUNT := $(if $(BIND_DIR),-v "$(CURDIR)/$(BIND_DIR):/var/log/onion")

# This allows the test suite to be able to run without worrying about the underlying fs used by the container running the daemon (e.g. aufs-on-aufs), so long as the host running the container is running a supported fs.
# The volume will be cleaned up when the container is removed due to `--rm`.
# Note that `BIND_DIR` will already be set to `bundles` if `DOCKER_HOST` is not set (see above BIND_DIR line), in such case this will do nothing since `DOCKER_MOUNT` will already be set.
DOCKER_MOUNT := $(if $(DOCKER_MOUNT),$(DOCKER_MOUNT),-v "/var/log/onion")

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := onion-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))

# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif

DOCKER_RUN := docker run --rm -i $(DOCKER_FLAGS) $(DOCKER_MOUNT) --privileged $(DOCKER_ENVS) "$(DOCKER_IMAGE)"
DOCKER_RUN_CI := docker run --rm -i $(DOCKER_FLAGS) --entrypoint make --privileged $(DOCKER_ENVS) "$(DOCKER_IMAGE)"

all: build

build:
	go build ./...

clean:
	rm -rf logs

ci: dtest-build
	$(DOCKER_RUN_CI) test

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
		--net host \
		--name tor-router \
		jess/tor-router

dtest-build: logs
	docker build --rm --force-rm -t "$(DOCKER_IMAGE)" -f $(CURDIR)/Dockerfile.test $(CURDIR)

dtest: dtest-build
	$(DOCKER_RUN)

fmt:
	@gofmt -s -l . | grep -v vendor | tee /dev/stderr

lint:
	@golint ./... | grep -v vendor | tee /dev/stderr

logs:
	mkdir -p logs

shell: dtest-build
	$(DOCKER_RUN) bash

test: validate
	go test -v $(shell go list ./... | grep -v vendor)

validate: fmt lint vet

vet:
	go vet $(shell go list ./... | grep -v vendor)
