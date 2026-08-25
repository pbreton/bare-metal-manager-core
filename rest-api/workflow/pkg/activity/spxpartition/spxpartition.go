// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	cutil "github.com/NVIDIA/infra-controller/rest-api/common/pkg/util"
	cdb "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db"
	cdbm "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
	cdbp "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/paginator"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
)

type ManageSpxPartition struct {
	dbSession *cdb.Session
}

func NewManageSpxPartition(dbSession *cdb.Session) ManageSpxPartition {
	return ManageSpxPartition{dbSession: dbSession}
}

func (manager ManageSpxPartition) UpdateSpxPartitionsInDB(ctx context.Context, siteID uuid.UUID, inventory *corev1.SpxPartitionInventory) error {
	logger := log.With().Str("Activity", "UpdateSpxPartitionsInDB").Str("site_id", siteID.String()).Logger()
	site, err := cdbm.NewSiteDAO(manager.dbSession).GetByID(ctx, nil, siteID, nil, false)
	if err != nil {
		return err
	}
	if inventory == nil || inventory.InventoryStatus == corev1.InventoryStatus_INVENTORY_STATUS_FAILED {
		return nil
	}

	partitionDAO := cdbm.NewSpxPartitionDAO(manager.dbSession)
	existing, _, err := partitionDAO.GetAll(ctx, nil, cdbm.SpxPartitionFilterInput{SiteIDs: []uuid.UUID{site.ID}, IncludeMissingOnSite: true}, cdbp.PageInput{Limit: cutil.GetPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		return err
	}
	existingByID := make(map[uuid.UUID]*cdbm.SpxPartition, len(existing))
	for index := range existing {
		partition := &existing[index]
		existingByID[partition.ID] = partition
	}

	reportedIDs := map[uuid.UUID]bool{}
	if inventory.InventoryPage != nil {
		for _, value := range inventory.InventoryPage.ItemIds {
			if id, parseErr := uuid.Parse(value); parseErr == nil {
				reportedIDs[id] = true
			}
		}
	}

	for _, corePartition := range inventory.SpxPartitions {
		if corePartition == nil || corePartition.Id == nil {
			continue
		}
		id, parseErr := uuid.Parse(corePartition.Id.Value)
		if parseErr != nil {
			logger.Warn().Err(parseErr).Str("id", corePartition.Id.Value).Msg("invalid SPX Partition ID in inventory")
			continue
		}
		reportedIDs[id] = true
		cloudPartition, found := existingByID[id]
		if !found {
			if err := manager.importPartition(ctx, site, corePartition); err != nil {
				logger.Error().Err(err).Str("spx_partition_id", id.String()).Msg("failed to import Core SPX Partition")
			}
			continue
		}
		if err := manager.reconcilePartition(ctx, partitionDAO, cloudPartition, corePartition); err != nil {
			logger.Error().Err(err).Str("spx_partition_id", id.String()).Msg("failed to reconcile SPX Partition")
		}
	}

	isLastPage := inventory.InventoryPage == nil || inventory.InventoryPage.TotalPages == 0 || inventory.InventoryPage.CurrentPage == inventory.InventoryPage.TotalPages
	if !isLastPage {
		return nil
	}
	for id, partition := range existingByID {
		if reportedIDs[id] || partition.IsMissingOnSite || site.IsTimeWithinStaleInventoryThreshold(partition.Updated) {
			continue
		}
		missing := true
		if _, err := partitionDAO.Update(ctx, nil, cdbm.SpxPartitionUpdateInput{SpxPartitionID: id, IsMissingOnSite: &missing}); err != nil {
			logger.Error().Err(err).Str("spx_partition_id", id.String()).Msg("failed to mark SPX Partition absent from Core inventory")
		}
	}
	return nil
}

func (manager ManageSpxPartition) importPartition(ctx context.Context, site *cdbm.Site, corePartition *corev1.SpxPartition) error {
	if corePartition.Metadata == nil || corePartition.Metadata.Name == "" || corePartition.TenantOrganizationId == "" {
		return fmt.Errorf("core SPX Partition is missing required metadata or tenant organization ID")
	}
	tenant, err := manager.tenantForOrg(ctx, corePartition.TenantOrganizationId)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(corePartition.Id.Value)
	if err != nil {
		return err
	}
	var description *string
	if corePartition.Metadata.Description != "" {
		description = &corePartition.Metadata.Description
	}
	_, err = cdbm.NewSpxPartitionDAO(manager.dbSession).Create(ctx, nil, cdbm.SpxPartitionCreateInput{
		SpxPartitionID: id, Name: corePartition.Metadata.Name, Description: description,
		TenantOrg: corePartition.TenantOrganizationId, SiteID: site.ID, TenantID: tenant.ID,
		VNI: corePartition.Vni, Labels: cdbm.Labels{}, CreatedBy: tenant.CreatedBy,
	})
	return err
}

func (manager ManageSpxPartition) tenantForOrg(ctx context.Context, org string) (*cdbm.Tenant, error) {
	tenants, total, err := cdbm.NewTenantDAO(manager.dbSession).GetAll(ctx, nil, cdbm.TenantFilterInput{Orgs: []string{org}}, cdbp.PageInput{Limit: cutil.GetPtr(1)}, nil)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, fmt.Errorf("no cloud Tenant maps to Core tenant organization %q", org)
	}
	return &tenants[0], nil
}

func (manager ManageSpxPartition) reconcilePartition(ctx context.Context, partitionDAO cdbm.SpxPartitionDAO, cloud *cdbm.SpxPartition, core *corev1.SpxPartition) error {
	input := cdbm.SpxPartitionUpdateInput{SpxPartitionID: cloud.ID, Touch: true}
	if cloud.IsMissingOnSite {
		missing := false
		input.IsMissingOnSite = &missing
	}
	if core.Metadata != nil {
		if core.Metadata.Name != cloud.Name {
			input.Name = &core.Metadata.Name
		}
		var desiredDescription *string
		if core.Metadata.Description != "" {
			desiredDescription = &core.Metadata.Description
		}
		if (cloud.Description == nil) != (desiredDescription == nil) || (cloud.Description != nil && desiredDescription != nil && *cloud.Description != *desiredDescription) {
			input.Description = &desiredDescription
		}
	}
	if core.TenantOrganizationId != cloud.Org {
		tenant, err := manager.tenantForOrg(ctx, core.TenantOrganizationId)
		if err != nil {
			missing := true
			if _, updateErr := partitionDAO.Update(ctx, nil, cdbm.SpxPartitionUpdateInput{
				SpxPartitionID: cloud.ID, IsMissingOnSite: &missing,
			}); updateErr != nil {
				return fmt.Errorf("resolve Core tenant ownership: %v; hide stale cloud projection: %w", err, updateErr)
			}
			return err
		}
		input.TenantOrg = &core.TenantOrganizationId
		input.TenantID = &tenant.ID
	}
	if core.Vni != cloud.VNI {
		input.VNI = &core.Vni
	}
	_, err := partitionDAO.Update(ctx, nil, input)
	return err
}
