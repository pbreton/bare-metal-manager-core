// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
	metrics "github.com/NVIDIA/infra-controller/rest-api/workflow/internal/metrics"
	spxPartitionActivity "github.com/NVIDIA/infra-controller/rest-api/workflow/pkg/activity/spxpartition"
)

func TestUpdateSpxPartitionInventory(t *testing.T) {
	tests := []struct {
		name        string
		activityErr error
		wantErr     bool
	}{
		{name: "inventory reconciled"},
		{name: "inventory reconciliation failed", activityErr: errors.New("SPX inventory failure"), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var workflowSuite testsuite.WorkflowTestSuite
			env := workflowSuite.NewTestWorkflowEnvironment()

			var manager spxPartitionActivity.ManageSpxPartition
			var metricsManager metrics.ManageInventoryMetrics
			env.RegisterActivity(manager.UpdateSpxPartitionsInDB)
			env.RegisterActivity(metricsManager.RecordLatency)
			env.OnActivity(manager.UpdateSpxPartitionsInDB, mock.Anything, mock.Anything, mock.Anything).Return(test.activityErr)
			env.OnActivity(metricsManager.RecordLatency, mock.Anything, mock.Anything, "UpdateSpxPartitionInventory", test.wantErr, mock.Anything).Return(nil)

			env.ExecuteWorkflow(UpdateSpxPartitionInventory, uuid.NewString(), &corev1.SpxPartitionInventory{})
			require.True(t, env.IsWorkflowCompleted())
			if test.wantErr {
				require.ErrorContains(t, env.GetWorkflowError(), test.activityErr.Error())
			} else {
				require.NoError(t, env.GetWorkflowError())
			}
			env.AssertExpectations(t)
		})
	}
}
