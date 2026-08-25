// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
	cwm "github.com/NVIDIA/infra-controller/rest-api/workflow/internal/metrics"
	spxPartitionActivity "github.com/NVIDIA/infra-controller/rest-api/workflow/pkg/activity/spxpartition"
)

func UpdateSpxPartitionInventory(ctx workflow.Context, siteID string, inventory *corev1.SpxPartitionInventory) (err error) {
	logger := log.With().Str("Workflow", "UpdateSpxPartitionInventory").Str("site_id", siteID).Logger()
	started := workflow.Now(ctx)
	parsedSiteID, err := uuid.Parse(siteID)
	if err != nil {
		return fmt.Errorf("invalid site ID %q: %w", siteID, err)
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2, MaximumInterval: 30 * time.Second, MaximumAttempts: 2},
	})
	var manager spxPartitionActivity.ManageSpxPartition
	err = workflow.ExecuteActivity(ctx, manager.UpdateSpxPartitionsInDB, parsedSiteID, inventory).Get(ctx, nil)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to update SPX Partition inventory")
	}
	var metricsManager cwm.ManageInventoryMetrics
	metricsErr := workflow.ExecuteActivity(ctx, metricsManager.RecordLatency, parsedSiteID, "UpdateSpxPartitionInventory", err != nil, workflow.Now(ctx).Sub(started)).Get(ctx, nil)
	if metricsErr != nil {
		logger.Warn().Err(metricsErr).Msg("failed to record SPX Partition inventory latency")
	}
	return err
}
