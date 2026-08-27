#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DPS_DIR="$("${SCRIPT_DIR}/prepare-dps-simulator.sh")"
IMAGE_TAG="${1:?usage: build-dps-simulator.sh IMAGE_TAG}"
DPSCTL_IMAGE_TAG="${2:-dps/dpsctl:nico-dev}"
BMC_SIMULATOR_IMAGE_TAG="${3:-dps/dps-bmc-simulator:nico-dev}"
TLS_PROXY_IMAGE_TAG="${4:-dps/dps-tls-proxy:nico-dev}"

require_bin() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required binary: %s\n' "$1" >&2
    exit 1
  }
}

require_bin docker
require_bin go
require_bin jq

TOOLS_BIN="${DPS_DIR}/../dps-tools/bin"
mkdir -p "${TOOLS_BIN}"

install_go_tool() {
  local binary="$1"
  local package="$2"
  local version="$3"

  if [[ ! -x "${TOOLS_BIN}/${binary}" ]]; then
    GOBIN="${TOOLS_BIN}" go install "${package}@${version}"
  fi
}

install_go_tool buf github.com/bufbuild/buf/cmd/buf v1.70.0
install_go_tool protoc-gen-go \
  google.golang.org/protobuf/cmd/protoc-gen-go v1.36.11
install_go_tool protoc-gen-go-grpc \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.0
install_go_tool protoc-gen-grpc-gateway \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway v2.28.0
install_go_tool sqlc github.com/sqlc-dev/sqlc/cmd/sqlc v1.30.0
install_go_tool oapi-codegen \
  github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen v2.5.1
install_go_tool mockgen go.uber.org/mock/mockgen v0.6.0

(
  cd "${DPS_DIR}"
  PATH="${TOOLS_BIN}:${PATH}" buf generate api \
    --template "${SCRIPT_DIR}/dps-buf.gen.api.yaml"
  PATH="${TOOLS_BIN}:${PATH}" buf generate internal/telemetry/grpc/testing \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output internal/telemetry/grpc/testing
  PATH="${TOOLS_BIN}:${PATH}" buf generate pkg/topology/pb \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output pkg/topology/pb
  PATH="${TOOLS_BIN}:${PATH}" buf generate internal/dps-bmc-simulator \
    --path internal/dps-bmc-simulator/api/v1/surrogate_model.proto \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output internal/dps-bmc-simulator
  PATH="${TOOLS_BIN}:${PATH}" buf generate \
    internal/dps-bmc-simulator/metrics/plugins/surrogate/pb \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output internal/dps-bmc-simulator/metrics/plugins/surrogate/pb
  mkdir -p internal/zapp-emulator/api/v1
  PATH="${TOOLS_BIN}:${PATH}" buf generate third_party/zapp/api \
    --path third_party/zapp/api/v1/zapp.proto \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output internal/zapp-emulator/api
  PATH="${TOOLS_BIN}:${PATH}" buf generate internal/zapp-emulator/api/v1 \
    --path internal/zapp-emulator/api/v1/emulator_options.proto \
    --template "${SCRIPT_DIR}/dps-buf.gen.go.yaml" \
    --output internal/zapp-emulator/api/v1
  PATH="${TOOLS_BIN}:${PATH}" sqlc generate --file sqlc.yaml
  PATH="${TOOLS_BIN}:${PATH}" oapi-codegen \
    -config internal/prs/cfg.yaml internal/prs/openapi.yaml
  mkdir -p internal/mocks/metrics/plugins/api
  PATH="${TOOLS_BIN}:${PATH}" mockgen \
    -source=internal/dps-bmc-simulator/metrics/plugins/api/types.go \
    -destination=internal/mocks/metrics/plugins/api/mock_types.go \
    -package=api
  PATH="${TOOLS_BIN}:${PATH}" mockgen \
    -source=internal/dps-bmc-simulator/metrics/plugins/api/interfaces.go \
    -destination=internal/mocks/metrics/plugins/api/mock_interfaces.go \
    -package=api
)

docker_arch="$(docker info --format '{{.Architecture}}')"
case "${docker_arch}" in
  amd64 | x86_64)
    go_arch="amd64"
    binary_arch="amd64_v1"
    ;;
  arm64 | aarch64)
    go_arch="arm64"
    binary_arch="arm64_v8.0"
    ;;
  *)
    printf 'unsupported Docker architecture: %s\n' "${docker_arch}" >&2
    exit 1
    ;;
esac

binary_dir="${DPS_DIR}/target/dist/dps-server_linux_${binary_arch}"
ctl_binary_dir="${DPS_DIR}/target/dist/dpsctl_linux_${binary_arch}"
bmc_simulator_binary_dir="${DPS_DIR}/target/dist/dps-bmc-simulator_linux_${binary_arch}"
bmc_simulator_mockups_dir="${DPS_DIR}/target/dist/dps-bmc-simulator-mockups"
tls_proxy_binary_dir="${DPS_DIR}/target/dist/dps-tls-proxy_linux_${binary_arch}"
mkdir -p "${binary_dir}"
mkdir -p "${ctl_binary_dir}"
mkdir -p "${bmc_simulator_binary_dir}"
mkdir -p "${bmc_simulator_mockups_dir}"
mkdir -p "${tls_proxy_binary_dir}"
cp "${SCRIPT_DIR}/dps-bmc-simulator-config.yaml" \
  "${DPS_DIR}/target/dist/dps-bmc-simulator-config.yaml"
cp -R "${DPS_DIR}/cmd/dps-bmc-simulator/mockups/." \
  "${bmc_simulator_mockups_dir}/"

# DPS 0.8.0 models each Grace CPU at 420 W, but CPU_0 in its GB200 Redfish
# mock advertises 300 W and rejects the model's default cap during topology
# activation. CPU_1 already advertises 420 W. Keep the staged test fixture
# aligned with the model without modifying the verified source checkout.
cpu_environment_metrics_path="gb200nvl/redfish/v1/Systems/HGX_Baseboard_0/Processors/CPU_0/EnvironmentMetrics/index.json"
source_cpu_environment_metrics="${DPS_DIR}/cmd/dps-bmc-simulator/mockups/${cpu_environment_metrics_path}"
cpu_environment_metrics="${bmc_simulator_mockups_dir}/${cpu_environment_metrics_path}"
cp -f "${source_cpu_environment_metrics}" "${cpu_environment_metrics}"
jq -e '.PowerLimitWatts.AllowableMax == 300' \
  "${cpu_environment_metrics}" >/dev/null
jq '.PowerLimitWatts.AllowableMax = 420' \
  "${cpu_environment_metrics}" >"${cpu_environment_metrics}.tmp"
mv "${cpu_environment_metrics}.tmp" "${cpu_environment_metrics}"

# The DGX_GB200 model's 900 W node minimum is divided equally between its two
# processor modules when DPS applies the idle policy. Both bundled Redfish
# mocks advertise a 500 W minimum and reject the resulting 450 W setpoint.
# Align only the staged fixtures with the model's valid node minimum.
for processor_module in 0 1; do
  processor_module_environment_metrics_path="gb200nvl/redfish/v1/Chassis/HGX_ProcessorModule_${processor_module}/EnvironmentMetrics/index.json"
  processor_module_environment_metrics="${bmc_simulator_mockups_dir}/${processor_module_environment_metrics_path}"
  jq -e '.PowerLimitWatts.AllowableMin == 500' \
    "${processor_module_environment_metrics}" >/dev/null
  jq '.PowerLimitWatts.AllowableMin = 450' \
    "${processor_module_environment_metrics}" \
    >"${processor_module_environment_metrics}.tmp"
  mv "${processor_module_environment_metrics}.tmp" \
    "${processor_module_environment_metrics}"
done

commit="$(git -C "${DPS_DIR}" rev-parse HEAD)"
build_date="$(date -u +%Y-%m-%d.%H:%M:%S)"
ldflags="-s -w -X nvidia.com/NVIDIA/dcpower/internal/metadata.Version=0.8.0 -X nvidia.com/NVIDIA/dcpower/internal/metadata.Commit=${commit} -X nvidia.com/NVIDIA/dcpower/internal/metadata.Date=${build_date}"

CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
  go build -trimpath -ldflags="-s -w" \
    -o "${tls_proxy_binary_dir}/dps-tls-proxy" \
    "${SCRIPT_DIR}/dps-tls-proxy.go"

(
  cd "${DPS_DIR}"
  CGO_ENABLED=0 go build -ldflags="${ldflags}" \
    -o "${TOOLS_BIN}/dpsctl" ./cmd/dpsctl/main.go
  CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
    go build -ldflags="${ldflags}" \
      -o "${binary_dir}/dps-server" ./cmd/dps-server/main.go
  CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
    go build -ldflags="${ldflags}" \
      -o "${ctl_binary_dir}/dpsctl" ./cmd/dpsctl/main.go
  CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
    go build -ldflags="${ldflags}" \
      -o "${bmc_simulator_binary_dir}/dps-bmc-simulator" \
      ./cmd/dps-bmc-simulator/main.go
  docker build --pull=false \
    --build-arg "BINARY_ARCH=${binary_arch}" \
    --tag "${IMAGE_TAG}" \
    --file "${SCRIPT_DIR}/Dockerfile.dps-simulator" .
  docker build --pull=false \
    --build-arg "BINARY_ARCH=${binary_arch}" \
    --tag "${DPSCTL_IMAGE_TAG}" \
    --file "${SCRIPT_DIR}/Dockerfile.dpsctl" .
  docker build --pull=false \
    --build-arg "BINARY_ARCH=${binary_arch}" \
    --tag "${BMC_SIMULATOR_IMAGE_TAG}" \
    --file "${SCRIPT_DIR}/Dockerfile.dps-bmc-simulator" .
  docker build --pull=false \
    --build-arg "BINARY_ARCH=${binary_arch}" \
    --tag "${TLS_PROXY_IMAGE_TAG}" \
    --file "${SCRIPT_DIR}/Dockerfile.dps-tls-proxy" .
)
