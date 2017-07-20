NAME=registrator
VERSION=$(shell cat VERSION)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
DEV_RUN_OPTS ?=-ttl 60 -ttl-refresh 30 -require-label -ip 127.0.0.1 -resync 30 eureka://127.0.0.1:8090/eureka/v2
PROD_RELEASE_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:latest
TEST_TAG=761584570493.dkr.ecr.us-east-1.amazonaws.com/registrator:$(BRANCH)

prep-dev: teardown
	docker run --rm --name reg_eureka -e "SERVICE_REGISTER=true" -td -p 8090:8080 netflixoss/eureka:1.1.147
	docker build -f Dockerfile.dev -t $(NAME):dev .

teardown:
	docker kill reg_eureka; true

dev: prep-dev dev-run teardown
dev-verbose: prep-dev dev-run-verbose teardown

dev-run:
	docker run -ti --rm \
		--net=host \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-e "FARGO_LOG_LEVEL=NOTICE"
		$(NAME):dev $(DEV_RUN_OPTS)

dev-run-verbose:
	docker run -ti --rm \
		--net=host \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-e "REGISTRATOR_LOG_LEVEL=DEBUG" \
		-e "FARGO_LOG_LEVEL=DEBUG"
		$(NAME):dev $(DEV_RUN_OPTS)

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
