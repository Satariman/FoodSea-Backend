#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FOODSEA_ENV="${FOODSEA_ENV:-dev}"
FOODSEA_HOST="${FOODSEA_HOST:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-foodsea}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
RELEASE_SHA="${RELEASE_SHA:-unknown}"
RELEASE_NAMESPACE="${RELEASE_NAMESPACE:-}"
IMAGE_DIGEST_CORE="${IMAGE_DIGEST_CORE:-unknown}"
IMAGE_DIGEST_OPTIMIZATION="${IMAGE_DIGEST_OPTIMIZATION:-unknown}"
IMAGE_DIGEST_ORDERING="${IMAGE_DIGEST_ORDERING:-unknown}"
IMAGE_DIGEST_ML="${IMAGE_DIGEST_ML:-unknown}"
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
if [[ -z "${RELEASE_NAMESPACE}" ]]; then
  RELEASE_NAMESPACE="${NS}"
fi

collect_diagnostics() {
  echo "collecting deployment diagnostics for namespace ${NS}..."
  kubectl -n "${NS}" get pods -o wide || true
  kubectl -n "${NS}" describe pod || true
  for dep in core-service optimization-service ordering-service ml-service; do
    kubectl -n "${NS}" logs "deployment/${dep}" --all-containers --tail=200 || true
    kubectl -n "${NS}" logs "deployment/${dep}" --all-containers --previous --tail=200 || true
  done
}

rollback_critical() {
  echo "rolling back critical deployments..."
  for dep in core-service optimization-service ordering-service ml-service; do
    kubectl -n "${NS}" rollout undo "deployment/${dep}" || true
  done
}

rollout_or_fail() {
  local resource="$1"
  if ! kubectl -n "${NS}" rollout status "${resource}" --timeout=600s; then
    collect_diagnostics
    rollback_critical
    exit 1
  fi
}

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

urlencode() {
  jq -rn --arg v "$1" '$v|@uri'
}

kubectl apply -f "${OVERLAY}/namespace.yaml"

CORE_DB_PASSWORD="${CORE_DB_PASSWORD:-$(secret_value core-secrets DB_PASSWORD)}"
OPTIMIZATION_DB_PASSWORD="${OPTIMIZATION_DB_PASSWORD:-$(secret_value optimization-secrets DB_PASSWORD)}"
ORDERING_DB_PASSWORD="${ORDERING_DB_PASSWORD:-$(secret_value ordering-secrets DB_PASSWORD)}"
JWT_SECRET="${JWT_SECRET:-$(secret_value core-secrets JWT_SECRET)}"
MINIO_ROOT_USER="${MINIO_ROOT_USER:-$(secret_value minio-secrets MINIO_ROOT_USER minioadmin)}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-$(secret_value minio-secrets MINIO_ROOT_PASSWORD)}"
EXISTING_GEMINI_API_KEY="$(kubectl -n "${NS}" get secret gemini-api-key -o "jsonpath={.data.api-key}" 2>/dev/null | base64 -d 2>/dev/null || true)"
GEMINI_API_KEY="${GEMINI_API_KEY:-${EXISTING_GEMINI_API_KEY}}"

CORE_DB_PASSWORD_URLENC="$(urlencode "${CORE_DB_PASSWORD}")"
OPTIMIZATION_DB_PASSWORD_URLENC="$(urlencode "${OPTIMIZATION_DB_PASSWORD}")"
ORDERING_DB_PASSWORD_URLENC="$(urlencode "${ORDERING_DB_PASSWORD}")"

create_secret core-secrets \
  --from-literal=DB_URL="postgres://postgres:${CORE_DB_PASSWORD_URLENC}@core-postgres:5432/core_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${CORE_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret optimization-secrets \
  --from-literal=DB_URL="postgres://postgres:${OPTIMIZATION_DB_PASSWORD_URLENC}@optimization-postgres:5432/optimization_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${OPTIMIZATION_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret ordering-secrets \
  --from-literal=DB_URL="postgres://postgres:${ORDERING_DB_PASSWORD_URLENC}@ordering-postgres:5432/ordering_db?sslmode=disable" \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD="${ORDERING_DB_PASSWORD}" \
  --from-literal=JWT_SECRET="${JWT_SECRET}"

create_secret minio-secrets \
  --from-literal=MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
  --from-literal=MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}"

create_secret gemini-api-key \
  --from-literal=api-key="${GEMINI_API_KEY}"

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

for job_name in migrate-core migrate-optimization migrate-ordering kafka-init-topics; do
  kubectl -n "${NS}" delete job "${job_name}" --ignore-not-found --wait=true
done

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

set +e
kubectl diff -f "${TMP_MANIFEST}" >/tmp/foodsea-kubectl-diff.txt
diff_exit=$?
set -e
# exit code 1 means diff found, >1 means real error
if [[ ${diff_exit} -gt 1 ]]; then
  echo "kubectl diff failed"
  cat /tmp/foodsea-kubectl-diff.txt || true
  exit ${diff_exit}
fi
cat /tmp/foodsea-kubectl-diff.txt || true

kubectl apply -f "${TMP_MANIFEST}"

kubectl -n "${NS}" wait --for=condition=complete job/migrate-core --timeout=600s
kubectl -n "${NS}" wait --for=condition=complete job/migrate-optimization --timeout=600s
kubectl -n "${NS}" wait --for=condition=complete job/migrate-ordering --timeout=600s
if ! kubectl -n "${NS}" wait --for=condition=complete job/kafka-init-topics --timeout=180s; then
  echo "warning: kafka-init-topics job did not complete in time; continuing deployment"
  kubectl -n "${NS}" logs job/kafka-init-topics --tail=200 || true
fi

kubectl -n "${NS}" rollout status statefulset/core-postgres --timeout=600s
kubectl -n "${NS}" rollout status statefulset/optimization-postgres --timeout=600s
kubectl -n "${NS}" rollout status statefulset/ordering-postgres --timeout=600s
rollout_or_fail deployment/core-service
rollout_or_fail deployment/optimization-service
rollout_or_fail deployment/ordering-service
rollout_or_fail deployment/ml-service

curl -fsS -H "Host: ${FOODSEA_HOST}" "${SMOKE_SCHEME}://${SMOKE_ADDR}/api/v1/categories" >/dev/null

report_path="${ROOT_DIR}/reports/deploy-${FOODSEA_ENV}-$(date +%Y%m%d-%H%M%S).json"
mkdir -p "$(dirname "${report_path}")"
cat >"${report_path}" <<EOF
{
  "commit_sha": "${RELEASE_SHA}",
  "namespace": "${RELEASE_NAMESPACE}",
  "image_tag": "${IMAGE_TAG}",
  "images": {
    "core": {"tag": "${IMAGE_REGISTRY}/core-service:${IMAGE_TAG}", "digest": "${IMAGE_DIGEST_CORE}"},
    "optimization": {"tag": "${IMAGE_REGISTRY}/optimization-service:${IMAGE_TAG}", "digest": "${IMAGE_DIGEST_OPTIMIZATION}"},
    "ordering": {"tag": "${IMAGE_REGISTRY}/ordering-service:${IMAGE_TAG}", "digest": "${IMAGE_DIGEST_ORDERING}"},
    "ml": {"tag": "${IMAGE_REGISTRY}/ml-service:${IMAGE_TAG}", "digest": "${IMAGE_DIGEST_ML}"}
  },
  "smoke_check": "passed"
}
EOF

echo "deploy report saved to ${report_path}"
echo "Deployment ${FOODSEA_ENV} is ready at ${SMOKE_SCHEME}://${FOODSEA_HOST}"
