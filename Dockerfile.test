FROM docker:1.12-dind

ENV PATH /go/bin:$PATH
ENV GOPATH /go
ENV GO15VENDOREXPERIMENT 1

RUN apk add --update \
	bash \
	curl \
	go \
	git \
	gcc \
	jq \
	libc-dev \
	libgcc \
	make \
	--repository https://dl-3.alpinelinux.org/alpine/edge/community/ \
	&& rm -rf /var/cache/apk/*

# install bats
RUN cd /tmp \
    && git clone https://github.com/sstephenson/bats.git \
    && ./bats/install.sh /usr/local \
	&& rm -rf /tmp/bats

# install golint
RUN go get github.com/golang/lint/golint \
	&& go install cmd/vet

COPY . /go/src/github.com/jessfraz/onion

WORKDIR /go/src/github.com/jessfraz/onion

RUN go build -o /usr/bin/onion .

ENTRYPOINT [ "dind" ]
CMD [ "hack/test.sh" ]
