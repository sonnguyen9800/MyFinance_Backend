VERSION := 1.0.0
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

.PHONY: build
build:
	go build -ldflags "-X my-finance-backend/version.Version=$(VERSION) \
		-X my-finance-backend/version.GitCommit=$(GIT_COMMIT) \
		-X my-finance-backend/version.GitBranch=$(GIT_BRANCH) \
		-X my-finance-backend/version.BuildTime=$(BUILD_TIME)" \
		-o bin/my-finance-backend.exe

.PHONY: run
run: build
	./bin/my-finance-backend.exe

.PHONY: clean
clean:
	rm -rf bin/
