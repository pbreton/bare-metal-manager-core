#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../../.." && pwd)"

DPS_VERSION="0.8.0"
DPS_COMMIT="15dd8ab0fd058d82e7303ca8781659c3c016f7d7"
DPS_URL="ssh://git@gitlab-master.nvidia.com:12051/dcgm/dcpower/dcpower.git"
DPS_DIR="${REPO_ROOT}/.devspace/dps-${DPS_VERSION}"

require_bin() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required binary: %s\n' "$1" >&2
    exit 1
  }
}

require_bin git

mkdir -p "${REPO_ROOT}/.devspace"

if [[ ! -d "${DPS_DIR}/.git" ]]; then
  if [[ -e "${DPS_DIR}" ]]; then
    printf 'refusing to replace incomplete DPS checkout: %s\n' "${DPS_DIR}" >&2
    exit 1
  fi

  git clone --filter=blob:none --depth 1 --branch "${DPS_VERSION}" \
    "${DPS_URL}" "${DPS_DIR}"
fi

actual_commit="$(git -C "${DPS_DIR}" rev-parse HEAD)"
if [[ "${actual_commit}" != "${DPS_COMMIT}" ]]; then
  printf 'DPS %s resolved to %s, expected %s\n' \
    "${DPS_VERSION}" "${actual_commit}" "${DPS_COMMIT}" >&2
  exit 1
fi

git -C "${DPS_DIR}" submodule update --init --depth 1

if [[ -n "$(git -C "${DPS_DIR}" status --porcelain --untracked-files=all)" ]]; then
  printf 'refusing to use modified DPS checkout: %s\n' "${DPS_DIR}" >&2
  exit 1
fi

for required_path in \
  cmd/dps-server/main.go \
  cmd/dps-server/Dockerfile \
  helm/dps/Chart.yaml \
  third_party/zapp/api/v1/zapp.proto; do
  if [[ ! -e "${DPS_DIR}/${required_path}" ]]; then
    printf 'DPS checkout is missing required path: %s\n' "${required_path}" >&2
    exit 1
  fi
done

printf '%s\n' "${DPS_DIR}"
