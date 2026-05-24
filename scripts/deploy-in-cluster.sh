#!/usr/bin/env bash
# Деплой K8s Manager в текущий kubectl-контекст одной командой:
#   make deploy-in-cluster
#   ./scripts/deploy-in-cluster.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${IMAGE:-k8s-manager:latest}"
NAMESPACE="${NAMESPACE:-k8s-manager}"
INGRESS_HOST="${INGRESS_HOST:-k8s-manager.local}"
HOSTS_IP="${HOSTS_IP:-127.0.0.1}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-180s}"

echo "==> Context: $(kubectl config current-context 2>/dev/null || echo '?')"

if command -v minikube >/dev/null 2>&1 && minikube status >/dev/null 2>&1; then
  echo "==> Building image in minikube Docker (${IMAGE})"
  eval "$(minikube docker-env)"
else
  echo "==> Building image with local Docker (${IMAGE})"
  echo "    (убедитесь, что кластер видит этот образ, или задайте IMAGE=<registry>/k8s-manager:tag)"
fi

docker build -t "${IMAGE}" "${ROOT}"

echo "==> Applying manifests (rbac, deployment, ingress-dev, prometheus, grafana)"
kubectl apply -f "${ROOT}/deploy/rbac.yaml"
kubectl apply -f "${ROOT}/deploy/deployment.yaml"
kubectl apply -f "${ROOT}/deploy/ingress-dev.yaml"
kubectl apply -f "${ROOT}/deploy/dev-cluster/prometheus.yaml"
kubectl apply -f "${ROOT}/deploy/grafana-provisioning.yaml"

echo "==> Waiting for rollout"
kubectl -n "${NAMESPACE}" rollout status "deployment/k8s-manager" --timeout="${ROLLOUT_TIMEOUT}"

INGRESS_ADDR="$(kubectl -n "${NAMESPACE}" get ingress k8s-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)"
POD="$(kubectl -n "${NAMESPACE}" get pods -l app=k8s-manager -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

echo ""
echo "=============================================="
echo " K8s Manager deployed in namespace: ${NAMESPACE}"
echo " Pod: ${POD:-?}"
echo " Ingress host: ${INGRESS_HOST}"
echo " Ingress address: ${INGRESS_ADDR:-pending (run: minikube tunnel)}"
echo "=============================================="
echo ""
echo "1) /etc/hosts (один раз):"
echo "   echo \"${HOSTS_IP} ${INGRESS_HOST}\" | sudo tee -a /etc/hosts"
echo ""
echo "2) minikube (Docker driver): в отдельном терминале"
echo "   minikube tunnel"
echo ""
echo "3) Откройте в браузере (HTTP, не HTTPS):"
echo "   http://${INGRESS_HOST}"
echo "   Логин: admin / secret"
echo ""
echo "Без Ingress:"
echo "   kubectl -n ${NAMESPACE} port-forward svc/k8s-manager 8080:8080"
echo "   http://localhost:8080"
echo ""
echo "Grafana (admin/admin):"
echo "   kubectl -n ${NAMESPACE} port-forward svc/grafana 3000:3000"
echo "   http://localhost:3000"
echo ""
