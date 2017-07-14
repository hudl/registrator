NAME=registrator
VERSION=$(shell cat VERSION)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
DEV_RUN_OPTS ?=-ttl 60 -ttl-refresh 30 -require-label -ip 127.0.0.1 -resync 30 eureka://127.0.0.1:8090/eureka/v2
PROD_RELEASE_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:latest
TEST_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:$(BRANCH)
DEPEND=github.com/Masterminds/glide

prep-dev: teardown
	docker kill reg_eureka; echo Stopped.
	docker run --rm --name reg_eureka -e "SERVICE_REGISTER=true" -td -p 8090:8080 netflixoss/eureka:1.1.147
	docker build -f Dockerfile.dev -t $(NAME):dev .

dev-run:
	docker run -ti --rm \
		--net=host \
		-v /var/run/docker.sock:/tmp/docker.sock \
		$(NAME):dev $(DEV_RUN_OPTS)

teardown:
	docker kill reg_eureka

dev: prep-dev dev-run teardown

build:
	mkdir -p build
	docker build -t $(NAME):$(VERSION) .
	docker save $(NAME):$(VERSION) | gzip -9 > build/$(NAME)_$(VERSION).tgz

test:
	docker build -t $(TEST_TAG) .
	docker push $(TEST_TAG)

release:
	docker build -t $(PROD_RELEASE_TAG) .
	docker push $(PROD_RELEASE_TAG)

depend:
	go get -v $(DEPEND)
	glide install