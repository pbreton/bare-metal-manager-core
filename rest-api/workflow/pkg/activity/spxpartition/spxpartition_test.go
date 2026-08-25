// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cutil "github.com/NVIDIA/infra-controller/rest-api/common/pkg/util"
	cdbm "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
	cdbp "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/paginator"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
	wutil "github.com/NVIDIA/infra-controller/rest-api/workflow/pkg/util"
)

func TestManageSpxPartitionReconcilesCoreInventory(t *testing.T) {
	ctx := context.Background()
	dbSession := wutil.TestInitDB(t)
	t.Cleanup(dbSession.Close)
	wutil.TestSetupSchema(t, dbSession)

	providerUser := wutil.TestBuildUser(t, dbSession, uuid.NewString(), []string{"provider-org"}, []string{"FORGE_PROVIDER_ADMIN"})
	provider := wutil.TestBuildInfrastructureProvider(t, dbSession, "provider", "provider-org", providerUser)
	site := wutil.TestBuildSite(t, dbSession, provider, "site", cdbm.SiteStatusRegistered, nil, providerUser)
	tenantUser := wutil.TestBuildUser(t, dbSession, uuid.NewString(), []string{"tenant-org"}, []string{"FORGE_TENANT_ADMIN"})
	tenant := wutil.TestBuildTenant(t, dbSession, "tenant", "tenant-org", nil, tenantUser)
	reassignedUser := wutil.TestBuildUser(t, dbSession, uuid.NewString(), []string{"reassigned-org"}, []string{"FORGE_TENANT_ADMIN"})
	reassignedTenant := wutil.TestBuildTenant(t, dbSession, "reassigned-tenant", "reassigned-org", nil, reassignedUser)

	partitionDAO := cdbm.NewSpxPartitionDAO(dbSession)
	existing, err := partitionDAO.Create(ctx, nil, cdbm.SpxPartitionCreateInput{
		SpxPartitionID: uuid.New(), Name: "old-name", TenantOrg: tenant.Org,
		SiteID: site.ID, TenantID: tenant.ID, VNI: 100, Labels: cdbm.Labels{"old": "label"}, CreatedBy: tenantUser.ID,
	})
	require.NoError(t, err)
	absent, err := partitionDAO.Create(ctx, nil, cdbm.SpxPartitionCreateInput{
		SpxPartitionID: uuid.New(), Name: "absent", TenantOrg: tenant.Org,
		SiteID: site.ID, TenantID: tenant.ID, VNI: 101, Labels: cdbm.Labels{}, CreatedBy: tenantUser.ID,
	})
	require.NoError(t, err)
	unmappedOwner, err := partitionDAO.Create(ctx, nil, cdbm.SpxPartitionCreateInput{
		SpxPartitionID: uuid.New(), Name: "unmapped-owner", TenantOrg: tenant.Org,
		SiteID: site.ID, TenantID: tenant.ID, VNI: 102, Labels: cdbm.Labels{}, CreatedBy: tenantUser.ID,
	})
	require.NoError(t, err)
	unchanged, err := partitionDAO.Create(ctx, nil, cdbm.SpxPartitionCreateInput{
		SpxPartitionID: uuid.New(), Name: "unchanged", TenantOrg: tenant.Org,
		SiteID: site.ID, TenantID: tenant.ID, VNI: 103, Labels: cdbm.Labels{}, CreatedBy: tenantUser.ID,
	})
	require.NoError(t, err)
	staleTime := time.Now().Add(-(cutil.DefaultInventoryReceiptInterval + cutil.StaleInventoryBuffer + time.Second))
	_, err = dbSession.DB.Exec("UPDATE spx_partition SET updated = ? WHERE id = ?", staleTime, absent.ID)
	require.NoError(t, err)
	_, err = dbSession.DB.Exec("UPDATE spx_partition SET updated = ? WHERE id = ?", staleTime, unchanged.ID)
	require.NoError(t, err)

	importedID := uuid.New()
	inventory := &corev1.SpxPartitionInventory{
		InventoryStatus: corev1.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		SpxPartitions: []*corev1.SpxPartition{
			{
				Id: &corev1.SpxPartitionId{Value: existing.ID.String()}, Vni: 200, TenantOrganizationId: reassignedTenant.Org,
				Metadata: &corev1.Metadata{Name: "new-name", Description: "updated"},
			},
			{
				Id: &corev1.SpxPartitionId{Value: importedID.String()}, Vni: 300, TenantOrganizationId: tenant.Org,
				Metadata: &corev1.Metadata{Name: "imported", Labels: []*corev1.Label{}},
			},
			{
				Id: &corev1.SpxPartitionId{Value: unmappedOwner.ID.String()}, Vni: 102, TenantOrganizationId: "unknown-org",
				Metadata: &corev1.Metadata{Name: "unmapped-owner"},
			},
			{
				Id: &corev1.SpxPartitionId{Value: unchanged.ID.String()}, Vni: unchanged.VNI, TenantOrganizationId: tenant.Org,
				Metadata: &corev1.Metadata{Name: unchanged.Name},
			},
		},
		InventoryPage: &corev1.InventoryPage{TotalPages: 1, CurrentPage: 1, ItemIds: []string{existing.ID.String(), importedID.String(), unmappedOwner.ID.String(), unchanged.ID.String()}},
	}

	manager := NewManageSpxPartition(dbSession)
	require.NoError(t, manager.UpdateSpxPartitionsInDB(ctx, site.ID, inventory))

	reconciled, err := partitionDAO.GetByID(ctx, nil, existing.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, "new-name", reconciled.Name)
	assert.Equal(t, uint32(200), reconciled.VNI)
	assert.Equal(t, cdbm.Labels{"old": "label"}, reconciled.Labels)
	assert.Equal(t, reassignedTenant.Org, reconciled.Org)
	assert.Equal(t, reassignedTenant.ID, reconciled.TenantID)

	imported, err := partitionDAO.GetByID(ctx, nil, importedID, nil)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, imported.TenantID)
	assert.Equal(t, uint32(300), imported.VNI)

	missing, err := partitionDAO.GetByID(ctx, nil, absent.ID, nil)
	require.NoError(t, err)
	assert.True(t, missing.IsMissingOnSite)
	unmapped, err := partitionDAO.GetByID(ctx, nil, unmappedOwner.ID, nil)
	require.NoError(t, err)
	assert.True(t, unmapped.IsMissingOnSite)
	refreshed, err := partitionDAO.GetByID(ctx, nil, unchanged.ID, nil)
	require.NoError(t, err)
	assert.False(t, refreshed.IsMissingOnSite)
	assert.True(t, refreshed.Updated.After(staleTime))
	visible, total, err := partitionDAO.GetAll(ctx, nil, cdbm.SpxPartitionFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cutil.GetPtr(cdbp.TotalLimit)}, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, visible, 3)

	reappearance := &corev1.SpxPartitionInventory{
		InventoryStatus: corev1.InventoryStatus_INVENTORY_STATUS_SUCCESS,
		SpxPartitions: []*corev1.SpxPartition{{
			Id: &corev1.SpxPartitionId{Value: absent.ID.String()}, Vni: absent.VNI,
			TenantOrganizationId: tenant.Org, Metadata: &corev1.Metadata{Name: absent.Name},
		}},
		InventoryPage: &corev1.InventoryPage{TotalPages: 1, CurrentPage: 1, ItemIds: []string{absent.ID.String()}},
	}
	require.NoError(t, manager.UpdateSpxPartitionsInDB(ctx, site.ID, reappearance))
	reappeared, err := partitionDAO.GetByID(ctx, nil, absent.ID, nil)
	require.NoError(t, err)
	assert.False(t, reappeared.IsMissingOnSite)
}
