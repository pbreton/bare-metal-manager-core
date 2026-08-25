// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
	cClient "github.com/NVIDIA/infra-controller/rest-api/site-workflow/pkg/grpc/client"
)

type ManageSpxPartitionInventory struct {
	config ManageInventoryConfig
}

func NewManageSpxPartitionInventory(config ManageInventoryConfig) ManageSpxPartitionInventory {
	return ManageSpxPartitionInventory{config: config}
}

func (manager *ManageSpxPartitionInventory) DiscoverSpxPartitionInventory(ctx context.Context) error {
	logger := log.With().Str("Activity", "DiscoverSpxPartitionInventory").Logger()
	inventoryImpl := manageInventoryImpl[*corev1.SpxPartitionId, *corev1.SpxPartition, *corev1.SpxPartitionInventory]{
		itemType: "SpxPartition", config: manager.config,
		internalFindIDs: spxPartitionFindIDs, internalFindByIDs: spxPartitionFindByIDs,
		internalPagedInventory: spxPartitionPagedInventory,
	}
	return inventoryImpl.CollectAndPublishInventory(ctx, &logger)
}

func spxPartitionFindIDs(ctx context.Context, grpcClient *cClient.CoreGrpcClient) ([]*corev1.SpxPartitionId, error) {
	response, err := grpcClient.GrpcServiceClient().FindSpxPartitionIds(ctx, &corev1.SpxPartitionSearchFilter{})
	if err != nil {
		return nil, err
	}
	return response.GetSpxPartitionIds(), nil
}

func spxPartitionFindByIDs(ctx context.Context, grpcClient *cClient.CoreGrpcClient, ids []*corev1.SpxPartitionId) ([]*corev1.SpxPartition, error) {
	response, err := grpcClient.GrpcServiceClient().FindSpxPartitionsByIds(ctx, &corev1.SpxPartitionsByIdsRequest{SpxPartitionIds: ids})
	if err != nil {
		return nil, err
	}
	return response.GetSpxPartitions(), nil
}

func spxPartitionPagedInventory(allIDs []*corev1.SpxPartitionId, partitions []*corev1.SpxPartition, input *pagedInventoryInput) *corev1.SpxPartitionInventory {
	itemIDs := make([]string, 0, len(allIDs))
	for _, id := range allIDs {
		itemIDs = append(itemIDs, id.GetValue())
	}
	inventory := &corev1.SpxPartitionInventory{
		SpxPartitions:   partitions,
		Timestamp:       &timestamppb.Timestamp{Seconds: time.Now().Unix()},
		InventoryStatus: input.status, StatusMsg: input.statusMessage, InventoryPage: input.buildPage(),
	}
	if inventory.InventoryPage != nil {
		inventory.InventoryPage.ItemIds = itemIDs
	}
	return inventory
}
