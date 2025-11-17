.PHONY: help build test up down logs

help:
	@echo "Commands:"
	@echo "  make build    - Build Docker image"
	@echo "  make test     - Run tests"
	@echo "  make up       - Start services"
	@echo "  make down     - Stop services"
	@echo "  make logs     - Show logs"

build:
	docker build -t deployment-api .

test:
	go test -v ./...

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f
