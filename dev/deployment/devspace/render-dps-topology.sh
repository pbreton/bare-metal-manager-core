#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

command -v jq >/dev/null 2>&1 || {
  printf 'missing required binary: jq\n' >&2
  exit 1
}

machine_ids="$(jq -ce '
  if type != "array" then
    error("machine IDs must be a JSON array")
  elif length == 0 then
    error("at least one machine ID is required")
  elif any(.[]; type != "string" or length == 0) then
    error("machine IDs must be non-empty strings")
  else
    unique | sort
  end
')"

jq -n --argjson machine_ids "${machine_ids}" '
  {
    Entities: ([
      {
        Type: "PowerDomain",
        Name: "nico-dev-power-domain",
        OperatingLimit: {PowerValue: {Value: 100000, Type: "W"}}
      },
      {
        Type: "PowerDistribution",
        Model: "FloorPDU95",
        Name: "nico-dev-pdu"
      },
      {
        Type: "PowerDistribution",
        Model: "RackPDU95_57500W",
        Name: "nico-dev-rpdu"
      }
    ] + ($machine_ids | map({
      Type: "ComputerSystem",
      Model: "DGX_GB200",
      Name: .,
      Redfish: {
        URL: "https://nico-dev-bmc.nico-dps.svc.cluster.local",
        SecretName: "nico-dev-bmc"
      }
    }))),
    Topology: {
      Name: "nico-dev",
      Entities: (
        ($machine_ids | map({Name: ., Policy: "low"})) +
        [
          {Name: "nico-dev-rpdu", Children: $machine_ids},
          {Name: "nico-dev-pdu", Children: ["nico-dev-rpdu"]},
          {
            Name: "nico-dev-power-domain",
            Children: ["nico-dev-pdu"]
          }
        ]
      )
    },
    Policies: [
      {
        Name: "low",
        Limits: [
          {ElementType: "Node", PowerLimit: {Percentage: 60}}
        ]
      },
      {
        Name: "balanced",
        Limits: [
          {ElementType: "Node", PowerLimit: {Percentage: 80}}
        ]
      },
      {
        Name: "performance",
        Limits: [
          {ElementType: "Node", PowerLimit: {Percentage: 100}}
        ]
      }
    ]
  }
'
