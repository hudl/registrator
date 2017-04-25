NAME=registrator
VERSION=$(shell cat VERSION)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
DEV_RUN_OPTS ?= -ttl 60 -ttl-refresh 30 -require-label -ip 127.0.0.1  eureka://localhost:8090/eureka/v2
PROD_RELEASE_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:latest
TEST_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:$(BRANCH)

dev:
	docker kill reg_eureka; echo Stopped.
	docker run --rm --name reg_eureka -e "SERVICE_REGISTER=true" -td -p 8090:8080 netflixoss/eureka:1.1.147
	docker build -f Dockerfile.dev -t $(NAME):dev .
	docker run --rm \
		--net=host \
		-v /var/run/docker.sock:/tmp/docker.sock \
		$(NAME):dev /bin/registrator $(DEV_RUN_OPTS)
	docker kill reg_eureka

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

