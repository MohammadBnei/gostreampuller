# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOTIDY=$(GOCMD) mod tidy
GOCLEAN=$(GOCMD) clean
BINARY_NAME=home-go-api-template
MAIN_PATH=.

ifneq (,$(wildcard ./.env))
	include .env
	export $(shell sed 's/=.*//' .env)
endif

# Linting
GOLINT=golangci-lint

.PHONY: all build test clean tidy test-ci lint dev

all: tidy lint build test


build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)

test:
	$(GOTEST) -v ./...

test-ci:
	$(GOTEST) -v ./... -coverprofile=coverage.out
	$(GOCMD) tool cover -func=coverage.out

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

tidy:
	$(GOTIDY)

lint:
	$(GOLINT) run

lint-fix:
	$(GOLINT) run --fix

run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)
	./$(BINARY_NAME)

# Docker commands
docker-build:
	docker build -t home-go-api-template:latest .

docker-run:
	docker run -p 8080:8080 home-go-api-template:latest

dev:
	gowatch

swag:
	swag fmt
	swag init