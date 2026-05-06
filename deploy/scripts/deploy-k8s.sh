#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FOODSEA_ENV="${FOODSEA_ENV:-dev}"
FOODSEA_HOST="${FOODSEA_HOST:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-foodsea}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
SMOKE_SCHEME="${SMOKE_SCHEME:-http}"
SMOKE_ADDR="${SMOKE_ADDR:-127.0.0.1}"

case "${FOODSEA_ENV}" in
  dev|prod) ;;
  *) echo "FOODSEA_ENV must be dev or prod"; exit 1 ;;
esac

if [[ -z "${FOODSEA_HOST}" ]]; then
  if [[ "${FOODSEA_ENV}" == "dev" ]]; then
    FOODSEA_HOST="dev.127.0.0.1.nip.io"
  else
    FOODSEA_HOST="api.127.0.0.1.nip.io"
  fi
fi

NS="foodsea-${FOODSEA_ENV}"
OVERLAY="${ROOT_DIR}/deploy/k8s/overlays/${FOODSEA_ENV}"
TMP_MANIFEST="$(mktemp)"
trap 'rm -f "${TMP_MANIFEST}"' EXIT

secret_value() {
  local name="$1"
  local key="$2"
  local fallback="${3:-}"
  local existing
  existing="$(kubectl -n "${NS}" get secret "${name}" -o "jsonpath={.data.${key}}" 2>/dev/null | base64 -d 2>/dev/null || true)"
  if [[ -n "${existing}" ]]; then
    printf "%s" "${existing}"
  elif [[ -n "${fallback}" ]]; then
    printf "%s" "${fallback}"
  else
    openssl rand -base64 32 | tr -d '\n'
  fi
}

create_secret() {
  local name="$1"
  shift
  kubectl -n "${NS}" create secret generic "${name}" "$@" --dry-run=client -o yaml | kubectl apply -f -
}

kubectl apply -f "${OVERLAY}/namespace.yaml"

CORE_DB_PASSWORD="${CORE_DB_PASSWORD:-$(secret_value core-secrets DB_PASSWORD)}"
OPTIMIZATION_DB_PASSWORD="${OPTIMIZATION_DB_PASSWORD:-$(secret_value optimization-secrets DB_PASSWORD)}"
ORDERING_DB_PASSWORD="${ORDERING_DB_PASSWORD:-$(secret_value ordering-secrets DB_PASSWORD)}"
JWT_SECRET="${JWT_SECRET:-$(secret_value core-secrets JWT_SECRET)}"
MINIO_ROOT_USER="${MINIO_ROOT_USER:-$(secret_value minio-secrets MINIO_ROOT_USER minioadmin)}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-$(secret_value minio-secrets MINIO_ROOT_PASSWORD)}"

create_secret core-secrets \
  --from-literal=DB_URL="postgres://postgres:${CORE_DB_PASSWORD}@core-postgres:5432/core_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${CORE_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret optimization-secrets \
  --from-literal=DB_URL="postgres://postgres:${OPTIMIZATION_DB_PASSWORD}@optimization-postgres:5432/optimization_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${OPTIMIZATION_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret ordering-secrets \
  --from-literal=DB_URL="postgres://postgres:${ORDERING_DB_PASSWORD}@ordering-postgres:5432/ordering_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${ORDERING_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret minio-secrets \
  --from-literal=MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
  --from-literal=MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}"

if [[ -n "${REGISTRY_USERNAME:-}" && -n "${REGISTRY_PASSWORD:-}" ]]; then
  kubectl -n "${NS}" create secret docker-registry registry-credentials \
    --docker-server="${REGISTRY_SERVER:-ghcr.io}" \
    --docker-username="${REGISTRY_USERNAME}" \
    --docker-password="${REGISTRY_PASSWORD}" \
    --dry-run=client -o yaml | kubectl apply -f -
else
  kubectl -n "${NS}" create secret docker-registry registry-credentials \
    --docker-server=example.invalid \
    --docker-username=unused \
    --docker-password=unused \
    --dry-run=client -o yaml | kubectl apply -f -
fi

kubectl -n "${NS}" delete job -l app.kubernetes.io/component=migration --ignore-not-found

kubectl kustomize "${OVERLAY}" > "${TMP_MANIFEST}"
sed -i.bak \
  -e "s#dev.foodsea.example.com#${FOODSEA_HOST}#g" \
  -e "s#api.foodsea.example.com#${FOODSEA_HOST}#g" \
  -e "s#foodsea.local#${FOODSEA_HOST}#g" \
  -e "s#image: core-service:latest#image: ${IMAGE_REGISTRY}/core-service:${IMAGE_TAG}#g" \
  -e "s#image: optimization-service:latest#image: ${IMAGE_REGISTRY}/optimization-service:${IMAGE_TAG}#g" \
  -e "s#image: ordering-service:latest#image: ${IMAGE_REGISTRY}/ordering-service:${IMAGE_TAG}#g" \
  -e "s#image: ml-service:latest#image: ${IMAGE_REGISTRY}/ml-service:${IMAGE_TAG}#g" \
  "${TMP_MANIFEST}"

kubectl apply -f "${TMP_MANIFEST}"

kubectl -n "${NS}" wait --for=condition=complete job/migrate-core --timeout=180s
kubectl -n "${NS}" wait --for=condition=complete job/migrate-optimization --timeout=180s
kubectl -n "${NS}" wait --for=condition=complete job/migrate-ordering --timeout=180s
kubectl -n "${NS}" wait --for=condition=complete job/kafka-init-topics --timeout=180s

kubectl -n "${NS}" rollout status statefulset/core-postgres --timeout=180s
kubectl -n "${NS}" rollout status statefulset/optimization-postgres --timeout=180s
kubectl -n "${NS}" rollout status statefulset/ordering-postgres --timeout=180s
kubectl -n "${NS}" rollout status deployment/core-service --timeout=240s
kubectl -n "${NS}" rollout status deployment/optimization-service --timeout=240s
kubectl -n "${NS}" rollout status deployment/ordering-service --timeout=240s
kubectl -n "${NS}" rollout status deployment/ml-service --timeout=240s

curl -fsS -H "Host: ${FOODSEA_HOST}" "${SMOKE_SCHEME}://${SMOKE_ADDR}/api/v1/categories" >/dev/null

echo "Deployment ${FOODSEA_ENV} is ready at ${SMOKE_SCHEME}://${FOODSEA_HOST}"
