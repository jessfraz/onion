FROM docker:dind

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN apk add --update \
	bash \
	go \
	git \
	gcc \
	libc-dev \
	libgcc \
	make \
	&& rm -rf /var/cache/apk/*

COPY . /go/src/github.com/jfrazelle/onion

WORKDIR /go/src/github.com/jfrazelle/onion

RUN go get -d -v github.com/jfrazelle/onion \
	&& go build -o /usr/bin/onion .

ENTRYPOINT [ "dind" ]
CMD [ "hack/test.sh" ]
