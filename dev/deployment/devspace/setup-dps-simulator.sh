#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"

DPS_NAMESPACE="nico-dps"
REST_NAMESPACE="nico-rest"
DPS_FORWARD_PORT="${LOCAL_DEV_DPS_FORWARD_PORT:-18551}"
API_FORWARD_PORT="${LOCAL_DEV_REST_API_FORWARD_PORT:-18388}"
KEYCLOAK_FORWARD_PORT="${LOCAL_DEV_KEYCLOAK_FORWARD_PORT:-18082}"
WORK_DIR="${LOCAL_DEV_DPS_WORK_DIR:-${HOME}/Developer/_agent-tmp/devspace-dps}"
DPSCTL="${REPO_ROOT}/.devspace/dps-tools/bin/dpsctl"
MACHINE_IDS_JSON="${LOCAL_DEV_DPS_MACHINE_IDS_JSON:-}"

dps_forward_pid=""
api_forward_pid=""
keycloak_forward_pid=""

cleanup() {
  for pid in "${dps_forward_pid}" "${api_forward_pid}" "${keycloak_forward_pid}"; do
    if [[ -n "${pid}" ]]; then
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  done
}

require_bin() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required binary: %s\n' "$1" >&2
    exit 1
  }
}

run_dpsctl() {
  "${DPSCTL}" \
    --host localhost \
    --port "${DPS_FORWARD_PORT}" \
    --insecure \
    --log-file "${WORK_DIR}/dpsctl.log" \
    "$@"
}

require_ok_response() {
  local operation="$1"
  local response="$2"

  if ! jq -e '.status.ok == true' <<<"${response}" >/dev/null; then
    printf 'DPS %s did not succeed:\n%s\n' "${operation}" "${response}" >&2
    exit 1
  fi
}

wait_for_http() {
  local name="$1"
  local url="$2"

  for _ in {1..120}; do
    if curl --fail --silent --max-time 5 "${url}" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  printf '%s did not become reachable: %s\n' "${name}" "${url}" >&2
  exit 1
}

resource_group_exists() {
  local resource_group="$1"
  local inactive_groups active_groups

  inactive_groups="$(run_dpsctl resource-group list)"
  active_groups="$(run_dpsctl resource-group list --active)"
  jq -e --arg resource_group "${resource_group}" '
    .. | objects |
    select((.groupName? // .group_name? // .name? // .Name? // "") == $resource_group)
  ' <<<"${inactive_groups}" >/dev/null || jq -e --arg resource_group "${resource_group}" '
    .. | objects |
    select((.groupName? // .group_name? // .name? // .Name? // "") == $resource_group)
  ' <<<"${active_groups}" >/dev/null
}

wait_for_vpc_status() {
  local vpc_id="$1"
  local expected_status="$2"
  local response status

  for _ in {1..120}; do
    response="$(curl --fail --silent --max-time 5 \
      "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/vpc/${vpc_id}" \
      -H "Authorization: Bearer ${token}")"
    status="$(jq -er '.status' <<<"${response}")"
    if [[ "${status}" == "${expected_status}" ]]; then
      return
    fi
    if [[ "${status}" == "Error" ]]; then
      printf 'VPC %s entered Error state:\n%s\n' "${vpc_id}" "${response}" >&2
      exit 1
    fi
    sleep 2
  done
  printf 'VPC %s did not reach %s\n' "${vpc_id}" "${expected_status}" >&2
  exit 1
}

trap cleanup EXIT INT TERM

require_bin curl
require_bin jq
require_bin kubectl
if [[ ! -x "${DPSCTL}" ]]; then
  printf 'missing DPS CLI built by the dps-simulator profile: %s\n' \
    "${DPSCTL}" >&2
  exit 1
fi

mkdir -p "${WORK_DIR}"

kubectl rollout status statefulset/dps-simulator-server \
  -n "${DPS_NAMESPACE}" --timeout=300s >/dev/null
kubectl rollout status deployment/bmc-gb200-simulator \
  -n "${DPS_NAMESPACE}" --timeout=300s >/dev/null
kubectl rollout status deployment/dps-tls-proxy \
  -n "${DPS_NAMESPACE}" --timeout=300s >/dev/null
kubectl rollout status deployment/nico-rest-api \
  -n "${REST_NAMESPACE}" --timeout=300s >/dev/null

kubectl port-forward --address 127.0.0.1 -n "${DPS_NAMESPACE}" \
  service/dps-simulator-server-grpc "${DPS_FORWARD_PORT}:8080" \
  >"${WORK_DIR}/dps-port-forward.log" 2>&1 &
dps_forward_pid=$!
kubectl port-forward --address 127.0.0.1 -n "${REST_NAMESPACE}" \
  service/nico-rest-api "${API_FORWARD_PORT}:8388" \
  >"${WORK_DIR}/api-port-forward.log" 2>&1 &
api_forward_pid=$!
kubectl port-forward --address 127.0.0.1 -n "${REST_NAMESPACE}" \
  service/keycloak "${KEYCLOAK_FORWARD_PORT}:8082" \
  >"${WORK_DIR}/keycloak-port-forward.log" 2>&1 &
keycloak_forward_pid=$!

wait_for_http "Keycloak" \
  "http://localhost:${KEYCLOAK_FORWARD_PORT}/realms/nico-dev"
wait_for_http "REST API" "http://localhost:${API_FORWARD_PORT}/healthz"

dps_ready=0
for _ in {1..120}; do
  if run_dpsctl server-version >/dev/null 2>&1; then
    dps_ready=1
    break
  fi
  sleep 1
done
if [[ "${dps_ready}" != "1" ]]; then
  printf 'DPS simulator did not become reachable on port %s\n' \
    "${DPS_FORWARD_PORT}" >&2
  exit 1
fi

token="$(curl --fail --silent --max-time 5 -X POST \
  "http://localhost:${KEYCLOAK_FORWARD_PORT}/realms/nico-dev/protocol/openid-connect/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'client_id=nico-api' \
  -d 'client_secret=nico-local-secret' \
  -d 'grant_type=password' \
  -d 'username=admin@example.com' \
  -d 'password=adminpassword' | jq -er '.access_token')"
site_id="$(kubectl get configmap nico-rest-site-agent-config \
  -n "${REST_NAMESPACE}" -o jsonpath='{.data.CLUSTER_ID}')"
site_response="$(curl --fail-with-body --silent --max-time 10 -X PATCH \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/site/${site_id}" \
  -H "Authorization: Bearer ${token}" \
  -H 'Content-Type: application/json' \
  -d '{"capabilities":{"dpsPowerManagement":true}}')"
if ! jq -e '.capabilities.dpsPowerManagement == true' \
    <<<"${site_response}" >/dev/null; then
  printf 'REST Site did not enable DPS power management:\n%s\n' \
    "${site_response}" >&2
  exit 1
fi
if [[ -n "${MACHINE_IDS_JSON}" ]]; then
  machines="${MACHINE_IDS_JSON}"
else
  machines="$(curl --fail --silent --max-time 10 \
    "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/machine?siteId=${site_id}&isMissingOnSite=false&pageSize=100" \
    -H "Authorization: Bearer ${token}")"
fi
machine_ids="$(jq -ce '
  if type != "array" then
    error("machine ID input is not an array")
  else
    [.[] | if type == "object" then .id else . end |
      select(type == "string" and length > 0)] | unique | sort
  end |
  if length == 0 then error("machine ID input is empty") else . end
' <<<"${machines}")"

topology_file="${WORK_DIR}/topology.json"
"${SCRIPT_DIR}/render-dps-topology.sh" <<<"${machine_ids}" >"${topology_file}"
run_dpsctl topology validate "${topology_file}" >/dev/null

inactive_topologies="$(run_dpsctl topology list)"
active_topologies="$(run_dpsctl topology list --active)"
topology_exists=0
if jq -e '
  .. | objects |
  select((.topologyName? // .topology_name? // .name? // .Name? // "") == "nico-dev")
' <<<"${inactive_topologies}" >/dev/null || jq -e '
  .. | objects |
  select((.topologyName? // .topology_name? // .name? // .Name? // "") == "nico-dev")
' <<<"${active_topologies}" >/dev/null; then
  topology_exists=1
fi
if jq -e '
  .. | objects |
  select((.topologyName? // .topology_name? // .name? // .Name? // "") == "nico-dev")
' <<<"${active_topologies}" >/dev/null; then
  response="$(run_dpsctl topology deactivate --topology nico-dev)"
  require_ok_response "topology deactivation" "${response}"
fi
if [[ "${topology_exists}" == "1" ]]; then
  response="$(run_dpsctl topology remove --topology nico-dev)"
  require_ok_response "topology removal" "${response}"
fi
response="$(run_dpsctl topology import "${topology_file}")"
require_ok_response "topology import" "${response}"
activation="$(run_dpsctl topology activate \
  --topology nico-dev \
  --replace-topology \
  --ping-hosts=false)"
if ! jq -e '
  (.nodeStatuses // .node_statuses // {}) as $node_statuses |
  (.status.ok == true) and
  ([$node_statuses[]?.status.ok] | length > 0 and all)
' <<<"${activation}" >/dev/null; then
  printf 'DPS topology activation did not succeed for every machine:\n%s\n' \
    "${activation}" >&2
  exit 1
fi

printf 'DPS simulator topology nico-dev is active with %s NICo machine IDs\n' \
  "$(jq 'length' <<<"${machine_ids}")"

tenant_id="$(curl --fail-with-body --silent --max-time 10 \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/tenant/current" \
  -H "Authorization: Bearer ${token}" | jq -er '.id')"
allocation_name="dps-e2e-site-allocation"
allocation_id="$(curl --fail-with-body --silent --max-time 10 \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/allocation?siteId=${site_id}&pageSize=100" \
  -H "Authorization: Bearer ${token}" | jq -r \
  --arg name "${allocation_name}" \
  --arg tenant_id "${tenant_id}" \
  '.[] | select(.name == $name and .tenantId == $tenant_id) | .id' | head -n 1)"
if [[ -z "${allocation_id}" ]]; then
  ip_block_name="dps-e2e-provider-ip-block"
  ip_block_id="$(curl --fail-with-body --silent --max-time 10 \
    "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/ipblock?siteId=${site_id}&pageSize=100" \
    -H "Authorization: Bearer ${token}" | jq -r \
    --arg name "${ip_block_name}" \
    '.[] | select(.name == $name and .tenantId == null) | .id' | head -n 1)"
  if [[ -z "${ip_block_id}" ]]; then
    ip_block_payload="$(jq -cn \
      --arg name "${ip_block_name}" \
      --arg site_id "${site_id}" \
      '{name: $name, siteId: $site_id, routingType: "Public", prefix: "198.18.0.0", prefixLength: 15, protocolVersion: "IPv4"}')"
    if ! ip_block_response="$(curl --fail-with-body --silent --max-time 30 -X POST \
      "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/ipblock" \
      -H "Authorization: Bearer ${token}" \
      -H 'Content-Type: application/json' \
      -d "${ip_block_payload}")"; then
      printf 'NICo IP Block creation failed:\n%s\n' "${ip_block_response}" >&2
      exit 1
    fi
    ip_block_id="$(jq -er '.id' <<<"${ip_block_response}")"
  fi

  allocation_payload="$(jq -cn \
    --arg name "${allocation_name}" \
    --arg tenant_id "${tenant_id}" \
    --arg site_id "${site_id}" \
    --arg ip_block_id "${ip_block_id}" \
    '{name: $name, tenantId: $tenant_id, siteId: $site_id,
      allocationConstraints: [{resourceType: "IPBlock", resourceTypeId: $ip_block_id,
        constraintType: "Reserved", constraintValue: 24}]}')"
  if ! allocation_response="$(curl --fail-with-body --silent --max-time 30 -X POST \
    "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/allocation" \
    -H "Authorization: Bearer ${token}" \
    -H 'Content-Type: application/json' \
    -d "${allocation_payload}")"; then
    printf 'NICo Allocation creation failed:\n%s\n' "${allocation_response}" >&2
    exit 1
  fi
  allocation_id="$(jq -er '.id' <<<"${allocation_response}")"
fi
printf 'NICo tenant %s has test Allocation %s for Site %s\n' \
  "${tenant_id}" "${allocation_id}" "${site_id}"

resource_group="nico-e2e-${site_id}"
existing_vpc_id="$(curl --fail-with-body --silent --max-time 10 \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/vpc?pageSize=100" \
  -H "Authorization: Bearer ${token}" | jq -r \
  --arg name 'dps-e2e-vpc' '.[] | select(.name == $name) | .id' | head -n 1)"
if [[ -n "${existing_vpc_id}" ]] || resource_group_exists "${resource_group}"; then
  printf 'DPS E2E VPC or resource group already exists; use a clean test deployment\n' >&2
  exit 1
fi

vpc_payload="$(jq -cn \
  --arg name 'dps-e2e-vpc' \
  --arg site_id "${site_id}" \
  --arg resource_group "${resource_group}" \
  '{name: $name, siteId: $site_id, networkVirtualizationType: "ETHERNET_VIRTUALIZER", powerResourceGroup: $resource_group}')"
if ! vpc_response="$(curl --fail-with-body --silent --max-time 30 -X POST \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/vpc" \
  -H "Authorization: Bearer ${token}" \
  -H 'Content-Type: application/json' \
  -d "${vpc_payload}")"; then
  printf 'NICo VPC creation failed:\n%s\n' "${vpc_response}" >&2
  exit 1
fi
vpc_id="$(jq -er '.id' <<<"${vpc_response}")"
if ! resource_group_exists "${resource_group}"; then
  printf 'NICo VPC creation did not create DPS resource group %s\n' \
    "${resource_group}" >&2
  exit 1
fi
wait_for_vpc_status "${vpc_id}" Ready

curl --fail-with-body --silent --max-time 30 -X DELETE \
  "http://localhost:${API_FORWARD_PORT}/v2/org/test-org/nico/vpc/${vpc_id}" \
  -H "Authorization: Bearer ${token}" >/dev/null
for _ in {1..60}; do
  if ! resource_group_exists "${resource_group}"; then
    printf 'NICo direct DPS VPC lifecycle succeeded for Tenant %s\n' \
      "${tenant_id}"
    exit 0
  fi
  sleep 1
done
printf 'NICo VPC deletion did not remove DPS resource group %s\n' \
  "${resource_group}" >&2
exit 1
