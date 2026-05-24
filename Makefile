DEV_PORT ?= 7777
# Драйвер minikube при первом старте из make dev (переопределение: make dev MINIKUBE_DRIVER=qemu2)
MINIKUBE_DRIVER ?= docker

.PHONY: build test test-js test-all run docker-build docker-run clean migrate-up migrate-down migrate-status dev dev-run dev-start dev-stop dev-restart deploy-in-cluster deploy-undeploy

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

# Один шаг: minikube + Prometheus + Grafana; затем HTTP-сервер (Postgres — bootstrap при старте).
# Порт: DEV_PORT (по умолчанию 7777).
dev dev-run:
	PORT=$(DEV_PORT) MINIKUBE_DRIVER=$(MINIKUBE_DRIVER) go run . dev-cluster run

dev-start:
	MINIKUBE_DRIVER=$(MINIKUBE_DRIVER) go run . dev-cluster start

dev-stop:
	go run . dev-cluster stop

dev-restart:
	go run . dev-cluster stop && PORT=$(DEV_PORT) MINIKUBE_DRIVER=$(MINIKUBE_DRIVER) go run . dev-cluster run

# Сборка образа + Deployment + Ingress в текущий kubectl-контекст (minikube: образ в minikube docker-env).
deploy-in-cluster:
	./scripts/deploy-in-cluster.sh

# Удалить и заново поднять в кластере.
deploy-redeploy: deploy-undeploy deploy-in-cluster

# Удалить release из кластера (namespace k8s-manager остаётся).
deploy-undeploy:
	kubectl delete -f deploy/grafana-provisioning.yaml --ignore-not-found
	kubectl delete -f deploy/dev-cluster/prometheus.yaml --ignore-not-found
	kubectl delete -f deploy/ingress-dev.yaml --ignore-not-found
	kubectl delete -f deploy/deployment.yaml --ignore-not-found
	kubectl delete -f deploy/rbac.yaml --ignore-not-found
