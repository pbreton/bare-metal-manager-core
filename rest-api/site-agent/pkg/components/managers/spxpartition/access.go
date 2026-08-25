// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	manager "github.com/NVIDIA/infra-controller/rest-api/site-agent/pkg/components/managers/managerapi"
	"github.com/NVIDIA/infra-controller/rest-api/site-agent/pkg/datatypes/elektratypes"
)

var ManagerAccess *manager.ManagerAccess

type API struct{}

func NewSpxPartitionManager(superForge *elektratypes.Elektra, superAPI *manager.ManagerAPI, superConf *manager.ManagerConf) *API {
	ManagerAccess = &manager.ManagerAccess{Data: &manager.ManagerData{EB: superForge}, API: superAPI, Conf: superConf}
	return &API{}
}
