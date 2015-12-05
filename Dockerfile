FROM alpine
MAINTAINER Jessica Frazelle <jess@docker.com>

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk update && apk add \
	ca-certificates \
	iptables \
	&& rm -rf /var/cache/apk/*

COPY . /go/src/github.com/jfrazelle/onion

RUN buildDeps=' \
		go \
		git \
		gcc \
		libc-dev \
		libgcc \
	' \
	set -x \
	&& apk update \
	&& apk add $buildDeps \
	&& cd /go/src/github.com/jfrazelle/onion \
	&& go get -d -v github.com/jfrazelle/onion \
	&& go build -o /usr/bin/onion . \
	&& apk del $buildDeps \
	&& rm -rf /var/cache/apk/* \
	&& rm -rf /go \
	&& echo "Build complete."


ENTRYPOINT [ "onion" ]
