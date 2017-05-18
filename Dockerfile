FROM alpine:3.5
ENTRYPOINT ["/bin/run-registrator.sh"]

CMD mkdir /logs
COPY . /go/src/github.com/gliderlabs/registrator
COPY ./run-registrator.sh /bin
CMD chmod 755 /bin/run-registrator.sh
RUN apk --no-cache add -t build-deps build-base go coreutils git bash \
	&& apk --no-cache add ca-certificates \
	&& cd /go/src/github.com/gliderlabs/registrator \
	&& export GOPATH=/go \
  && git config --global http.https://gopkg.in.followRedirects true \
	&& go get \
	&& go build -ldflags "-X main.Version=$(cat VERSION)" -o /bin/registrator \
	&& rm -rf /go \
	&& apk del --purge build-deps 
