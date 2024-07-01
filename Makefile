default: all

all: tidy lint test build

build:
	go build -ldflags="-X 'github.com/MartyHub/size-it/internal/monitoring.Version=development'" -race

db_up:
	./scripts/db_up.sh

db_init:
	./scripts/db_init.sh

db_down:
	./scripts/db_down.sh

install:
	go install github.com/cortesi/modd/cmd/modd@latest

lint:
	golangci-lint run

sqlc:
	rm -rf internal/db/sqlc/*.go
	podman run --rm --volume $(CURDIR):/src --workdir /src sqlc/sqlc:1.26.0 generate --file internal/db/sqlc.yaml

test:
	go test -race -timeout 10s ./...

tidy:
	go mod tidy

watch: db_up
	modd --file=.modd.conf

.PHONY: all build db_down db_init db_up install lint sqlc test tidy watch
