.PHONY: build run test clean docker-up docker-down lint

build:
		go build -o bin/server ./cmd/server

run:
		go run ./cmd/server

test:
		go test -v ./...

lint:
		golangci-lint run ./...

lint-fix:
		golangci-lint run --fix ./...
				
loadtest:
		go run loadtest.go

clean:
		rm -rf bin/

docker-up:
		docker-compose up --build

docker-down:
		docker-compose down

docker-clean:
		docker-compose down -v
