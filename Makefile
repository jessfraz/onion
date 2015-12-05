.PHONY: build dbuild drun dtor

all: build

build:
	go build ./...

dbuild:
	@docker build --rm --force-rm -t jess/onion .

drun:
	@docker run --rm -it \
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
