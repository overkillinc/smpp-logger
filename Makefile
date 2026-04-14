# Makefile for common developer tasks
.PHONY: run docker-build docker-run compose-up compose-down test fmt tidy

run:
	go run ./cmd/smpp-logger

docker-build:
	docker build -t ghcr.io/overkillinc/smpp-logger:local .

docker-run:
	docker run --rm -p 2775:2775 -p 8080:8080 \
		-e SMPP_LOGGER_SYSTEM_ID=smpp-logger \
		-e SMPP_LOGGER_PASSWORD=smpp-logger \
		ghcr.io/overkillinc/smpp-logger:local

compose-up:
	docker-compose up -d

compose-down:
	docker-compose down

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
