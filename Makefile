.PHONY: build test test-js test-all run docker-build docker-run clean migrate-up migrate-down migrate-status dev-run dev-start dev-stop dev-restart

build:
	go build -o k8s-manager .

test:
	go test ./...

test-js:
	node --test static/js/utils.test.mjs

test-all: test test-js

run:
	go run .

run-auth:
	AUTH_USER=admin AUTH_PASSWORD=secret go run .

docker-build:
	docker build -t k8s-manager:latest .

docker-run:
	docker run --rm -p 8080:8080 -v "$$HOME/.kube:/root/.kube:ro" \
		-e AUTH_USER=admin -e AUTH_PASSWORD=secret \
		k8s-manager:latest

clean:
	rm -f k8s-manager

hashpass:
	go run ./cmd/hashpass/main.go

migrate-up:
	go run ./cmd/migrate/main.go -action up

migrate-down:
	go run ./cmd/migrate/main.go -action down

migrate-status:
	go run ./cmd/migrate/main.go -action status

dev-run:
	go run . dev-cluster run

dev-start:
	go run . dev-cluster start

dev-stop:
	go run . dev-cluster stop

dev-restart:
	go run . dev-cluster stop && go run . dev-cluster run
