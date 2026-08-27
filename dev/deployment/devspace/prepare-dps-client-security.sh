#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"
TLS_DIR="${REPO_ROOT}/.devspace/dps-client-security"
CA_KEY="${TLS_DIR}/ca.key"
CA_CERT="${TLS_DIR}/ca.crt"
SERVER_KEY="${TLS_DIR}/tls.key"
SERVER_CERT="${TLS_DIR}/tls.crt"
SERVER_CSR="${TLS_DIR}/tls.csr"
OPENSSL_CONFIG="${SCRIPT_DIR}/dps-tls-openssl.cnf"
DPS_NAMESPACE="nico-dps"
REST_NAMESPACE="nico-rest"
DPS_TLS_SECRET="dps-tls-proxy-tls"
DPS_CLIENT_SECRET="dps-client-credentials"
DPS_DEVELOPMENT_TOKEN="nico-dps-development-token"

require_bin() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required binary: %s\n' "$1" >&2
    exit 1
  }
}

require_bin kubectl
require_bin openssl

mkdir -p "${TLS_DIR}"
chmod 0700 "${TLS_DIR}"

if [[ ! -s "${CA_KEY}" || ! -s "${CA_CERT}" ]] || \
    ! openssl x509 -checkend 86400 -noout -in "${CA_CERT}" >/dev/null 2>&1; then
  openssl req -x509 -newkey rsa:3072 -sha256 -nodes -days 3650 \
    -subj '/CN=NICo local DPS simulator CA' \
    -keyout "${CA_KEY}" -out "${CA_CERT}" >/dev/null 2>&1
fi

if [[ ! -s "${SERVER_KEY}" || ! -s "${SERVER_CERT}" ]] || \
    ! openssl x509 -checkend 86400 -noout -in "${SERVER_CERT}" >/dev/null 2>&1 || \
    ! openssl verify -CAfile "${CA_CERT}" "${SERVER_CERT}" >/dev/null 2>&1; then
  openssl req -new -newkey rsa:2048 -sha256 -nodes \
    -config "${OPENSSL_CONFIG}" \
    -keyout "${SERVER_KEY}" -out "${SERVER_CSR}" >/dev/null 2>&1
  openssl x509 -req -sha256 -days 30 \
    -in "${SERVER_CSR}" -CA "${CA_CERT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -extfile "${OPENSSL_CONFIG}" -extensions v3_req \
    -out "${SERVER_CERT}" >/dev/null 2>&1
fi

openssl verify -CAfile "${CA_CERT}" "${SERVER_CERT}" >/dev/null
openssl x509 -checkhost dps-tls-proxy.nico-dps.svc.cluster.local \
  -noout -in "${SERVER_CERT}" >/dev/null

kubectl create namespace "${DPS_NAMESPACE}" --dry-run=client -o yaml |
  kubectl apply -f - >/dev/null
kubectl create namespace "${REST_NAMESPACE}" --dry-run=client -o yaml |
  kubectl apply -f - >/dev/null

kubectl create secret tls "${DPS_TLS_SECRET}" \
  -n "${DPS_NAMESPACE}" \
  --cert="${SERVER_CERT}" --key="${SERVER_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl create secret generic "${DPS_CLIENT_SECRET}" \
  -n "${REST_NAMESPACE}" \
  --from-literal=token="${DPS_DEVELOPMENT_TOKEN}" \
  --from-file=ca.crt="${CA_CERT}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

printf 'Prepared DPS simulator TLS and client Secrets\n'
