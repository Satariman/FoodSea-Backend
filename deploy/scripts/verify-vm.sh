#!/usr/bin/env bash
set -euo pipefail

DEV_HOST="${DEV_HOST:-dev.127.0.0.1.nip.io}"
PROD_HOST="${PROD_HOST:-api.127.0.0.1.nip.io}"
SMOKE_ADDR="${SMOKE_ADDR:-127.0.0.1}"

echo "UFW:"
ufw status verbose

echo "fail2ban sshd:"
fail2ban-client status sshd

echo "Kubernetes:"
kubectl get nodes -o wide
kubectl -n ingress-nginx get pods -o wide

for env in dev prod; do
  ns="foodsea-${env}"
  echo "${ns}:"
  kubectl -n "${ns}" get pods,svc,ingress
done

curl -fsS -H "Host: ${DEV_HOST}" "http://${SMOKE_ADDR}/api/v1/categories" >/dev/null
curl -fsS -H "Host: ${PROD_HOST}" "http://${SMOKE_ADDR}/api/v1/categories" >/dev/null

echo "Dev and prod smoke checks passed."
