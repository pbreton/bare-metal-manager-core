// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workflow

import (
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/NVIDIA/infra-controller/rest-api/site-workflow/pkg/activity"
)

func DiscoverSpxPartitionInventory(ctx workflow.Context) error {
	logger := log.With().Str("Workflow", "DiscoverSpxPartitionInventory").Logger()
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 2 * time.Second, BackoffCoefficient: 2, MaximumInterval: 10 * time.Second, MaximumAttempts: 2},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	var inventoryManager activity.ManageSpxPartitionInventory
	if err := workflow.ExecuteActivity(ctx, inventoryManager.DiscoverSpxPartitionInventory).Get(ctx, nil); err != nil {
		logger.Error().Err(err).Msg("Failed to discover SPX Partition inventory")
		return err
	}
	return nil
}
