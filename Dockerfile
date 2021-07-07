FROM alpine:3.8

ENV GOPATH /go
WORKDIR /go/src/github.com/gliderlabs/registrator
COPY . .

RUN apk --no-cache add -t build-deps build-base go git \
	&& apk --no-cache add ca-certificates bash coreutils \
	&& cd /go/src/github.com/gliderlabs/registrator \
	&& export GOPATH=/go \
  	&& git config --global http.https://gopkg.in.followRedirects true

ARG OS="linux"
ARG ARCH="amd64"
ENV GOOS=${OS}
ENV GOARCH=${ARCH}

RUN go get

CMD ["go"]


#&& go build -ldflags "-X main.Version=$(cat VERSION)" -o /bin/registrator \