// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	"context"

	"go.temporal.io/sdk/client"

	sww "github.com/NVIDIA/infra-controller/rest-api/site-workflow/pkg/workflow"
)

const InventoryDefaultSchedule = "@every 3m"

func (api *API) RegisterCron() error {
	cronSchedule := InventoryDefaultSchedule
	if configured := ManagerAccess.Conf.EB.Temporal.TemporalInventorySchedule; configured != "" {
		cronSchedule = configured
	}
	_, err := ManagerAccess.Data.EB.Managers.Workflow.Temporal.Subscriber.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:           "inventory-spx-partition-" + ManagerAccess.Conf.EB.Temporal.TemporalSubscribeNamespace,
		TaskQueue:    ManagerAccess.Conf.EB.Temporal.TemporalSubscribeQueue,
		CronSchedule: cronSchedule,
	}, sww.DiscoverSpxPartitionInventory)
	return err
}
