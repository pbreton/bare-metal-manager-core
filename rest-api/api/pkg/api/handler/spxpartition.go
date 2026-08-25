// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/model"
	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/pagination"
	sc "github.com/NVIDIA/infra-controller/rest-api/api/pkg/client/site"
	auth "github.com/NVIDIA/infra-controller/rest-api/auth/pkg/authorization"
	cutil "github.com/NVIDIA/infra-controller/rest-api/common/pkg/util"
	cdb "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db"
	cdbm "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
	cdbp "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/paginator"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
)

type CreateSpxPartitionHandler struct {
	dbSession  *cdb.Session
	scp        *sc.ClientPool
	tracerSpan *cutil.TracerSpan
}

func NewCreateSpxPartitionHandler(dbSession *cdb.Session, scp *sc.ClientPool) CreateSpxPartitionHandler {
	return CreateSpxPartitionHandler{dbSession: dbSession, scp: scp, tracerSpan: cutil.NewTracerSpan()}
}

func (handler CreateSpxPartitionHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("SpxPartition", "Create", c, handler.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	if dbUser == nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}
	if ok, err := auth.ValidateOrgMembership(dbUser, org); !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership")
		}
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}
	if !auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole) {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}

	request := model.APISpxPartitionCreateRequest{}
	if err := c.Bind(&request); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse SPX Partition request data", nil)
	}
	if err := request.Validate(); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Error validating SPX Partition creation request", err)
	}
	tenant, err := common.GetTenantForOrg(ctx, nil, handler.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant for org", nil)
	}
	site, err := common.GetSiteFromIDString(ctx, nil, request.SiteID, handler.dbSession)
	if err != nil {
		if err == common.ErrInvalidID {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID", nil)
		}
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find Site with specified ID", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site", nil)
	}
	if site.Status != cdbm.SiteStatusRegistered {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request is not in Registered state", nil)
	}
	if _, err := cdbm.NewTenantSiteDAO(handler.dbSession).GetByTenantIDAndSiteID(ctx, nil, tenant.ID, site.ID, nil); err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant is not associated with Site specified in request", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to determine Tenant access to Site", nil)
	}

	partitionDAO := cdbm.NewSpxPartitionDAO(handler.dbSession)
	existing, total, err := partitionDAO.GetAll(ctx, nil, cdbm.SpxPartitionFilterInput{
		Names: []string{request.Name}, TenantIDs: []uuid.UUID{tenant.ID}, SiteIDs: []uuid.UUID{site.ID},
	}, cdbp.PageInput{}, nil)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to check SPX Partition name uniqueness", nil)
	}
	if total > 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusConflict, "Another SPX Partition with specified name already exists for Tenant at Site", validation.Errors{"id": errors.New(existing[0].ID.String())})
	}

	partitionID := uuid.New()
	stc, err := handler.scp.GetClientByID(site.ID)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}
	var corePartition corev1.SpxPartition
	if apiErr := common.ExecuteCoreGRPC(ctx, stc, corev1.Forge_CreateSpxPartition_FullMethodName, request.ToProto(partitionID, org), &corePartition, site.ID.String()); apiErr != nil {
		logAPIError(logger, apiErr, "failed to create SPX Partition via Core proxy")
		return cutil.NewAPIErrorResponse(c, apiErr.Code, apiErr.Message, nil)
	}
	if corePartition.Id == nil || corePartition.Id.Value != partitionID.String() {
		return cutil.NewAPIErrorResponse(c, http.StatusBadGateway, "Core returned an unexpected SPX Partition ID", nil)
	}
	if corePartition.Metadata == nil || corePartition.Metadata.Name == "" || corePartition.TenantOrganizationId != org {
		return cutil.NewAPIErrorResponse(c, http.StatusBadGateway, "Core returned an invalid SPX Partition", nil)
	}
	coreProjection := cdbm.SpxPartition{}
	coreProjection.FromProto(&corePartition)

	var partition *cdbm.SpxPartition
	err = cdb.WithTx(ctx, handler.dbSession, func(tx *cdb.Tx) error {
		var createErr error
		partition, createErr = partitionDAO.Create(ctx, tx, cdbm.SpxPartitionCreateInput{
			SpxPartitionID: partitionID, Name: coreProjection.Name, Description: coreProjection.Description,
			TenantOrg: coreProjection.Org, SiteID: site.ID, TenantID: tenant.ID, VNI: coreProjection.VNI,
			Labels: cdbm.Labels(request.Labels), CreatedBy: dbUser.ID,
		})
		return createErr
	})
	if err != nil {
		logger.Error().Err(err).Str("spx_partition_id", partitionID.String()).Msg("Core created SPX Partition but cloud persistence failed; inventory reconciliation will recover Core-owned fields")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Core created SPX Partition but REST persistence failed", nil)
	}
	partition.Site = site
	partition.Tenant = tenant
	return c.JSON(http.StatusCreated, model.NewAPISpxPartition(partition))
}

type GetAllSpxPartitionHandler struct {
	dbSession  *cdb.Session
	tracerSpan *cutil.TracerSpan
}

func NewGetAllSpxPartitionHandler(dbSession *cdb.Session) GetAllSpxPartitionHandler {
	return GetAllSpxPartitionHandler{dbSession: dbSession, tracerSpan: cutil.NewTracerSpan()}
}

func (handler GetAllSpxPartitionHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, _, handlerSpan := common.SetupHandler("SpxPartition", "GetAll", c, handler.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	if dbUser == nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}
	if ok, _ := auth.ValidateOrgMembership(dbUser, org); !ok {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}
	if !auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole) {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}
	tenant, err := common.GetTenantForOrg(ctx, nil, handler.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant for org", nil)
	}
	pageRequest := pagination.PageRequest{}
	if err := c.Bind(&pageRequest); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse pagination request", nil)
	}
	if err := pageRequest.Validate(cdbm.SpxPartitionOrderByFields); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate pagination request", err)
	}
	relations, errMessage := common.GetAndValidateQueryRelations(c.QueryParams(), cdbm.SpxPartitionRelatedEntities)
	if errMessage != "" {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errMessage, nil)
	}
	filter := cdbm.SpxPartitionFilterInput{TenantIDs: []uuid.UUID{tenant.ID}, SearchQuery: common.GetSearchQuery(c)}
	if name := c.QueryParam("name"); name != "" {
		filter.Names = []string{name}
	}
	if siteID := c.QueryParam("siteId"); siteID != "" {
		site, err := common.GetSiteFromIDString(ctx, nil, siteID, handler.dbSession)
		if err != nil {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid Site ID", nil)
		}
		if _, err := cdbm.NewTenantSiteDAO(handler.dbSession).GetByTenantIDAndSiteID(ctx, nil, tenant.ID, site.ID, nil); err != nil {
			if err == cdb.ErrDoesNotExist {
				return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Tenant does not have access to this Site", nil)
			}
			return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to determine Tenant access to Site", nil)
		}
		filter.SiteIDs = []uuid.UUID{site.ID}
	}
	if vniText := c.QueryParam("vni"); vniText != "" {
		vni, err := strconv.ParseUint(vniText, 10, 32)
		if err != nil {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid VNI", nil)
		}
		filter.VNIs = []uint32{uint32(vni)}
	}
	partitions, total, err := cdbm.NewSpxPartitionDAO(handler.dbSession).GetAll(ctx, nil, filter, cdbp.PageInput{Offset: pageRequest.Offset, Limit: pageRequest.Limit, OrderBy: pageRequest.OrderBy}, relations)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve SPX Partitions", nil)
	}
	response := make([]*model.APISpxPartition, 0, len(partitions))
	for index := range partitions {
		response = append(response, model.NewAPISpxPartition(&partitions[index]))
	}
	pageResponse := pagination.NewPageResponse(*pageRequest.PageNumber, *pageRequest.PageSize, total, pageRequest.OrderByStr)
	pageHeader, err := json.Marshal(pageResponse)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to generate pagination response header", nil)
	}
	c.Response().Header().Set(pagination.ResponseHeaderName, string(pageHeader))
	return c.JSON(http.StatusOK, response)
}

type GetSpxPartitionHandler struct {
	dbSession  *cdb.Session
	tracerSpan *cutil.TracerSpan
}

func NewGetSpxPartitionHandler(dbSession *cdb.Session) GetSpxPartitionHandler {
	return GetSpxPartitionHandler{dbSession: dbSession, tracerSpan: cutil.NewTracerSpan()}
}

func (handler GetSpxPartitionHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, _, handlerSpan := common.SetupHandler("SpxPartition", "Get", c, handler.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	if dbUser == nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}
	if ok, _ := auth.ValidateOrgMembership(dbUser, org); !ok {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}
	if !auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole) {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}
	tenant, err := common.GetTenantForOrg(ctx, nil, handler.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant for org", nil)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid SPX Partition ID", nil)
	}
	relations, errMessage := common.GetAndValidateQueryRelations(c.QueryParams(), cdbm.SpxPartitionRelatedEntities)
	if errMessage != "" {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, errMessage, nil)
	}
	partition, err := cdbm.NewSpxPartitionDAO(handler.dbSession).GetByID(ctx, nil, id, relations)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find SPX Partition with specified ID", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve SPX Partition", nil)
	}
	if partition.TenantID != tenant.ID {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "SPX Partition is not owned by current org's Tenant", nil)
	}
	if partition.IsMissingOnSite {
		return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find SPX Partition with specified ID", nil)
	}
	return c.JSON(http.StatusOK, model.NewAPISpxPartition(partition))
}

type DeleteSpxPartitionHandler struct {
	dbSession  *cdb.Session
	scp        *sc.ClientPool
	tracerSpan *cutil.TracerSpan
}

func NewDeleteSpxPartitionHandler(dbSession *cdb.Session, scp *sc.ClientPool) DeleteSpxPartitionHandler {
	return DeleteSpxPartitionHandler{dbSession: dbSession, scp: scp, tracerSpan: cutil.NewTracerSpan()}
}

func (handler DeleteSpxPartitionHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("SpxPartition", "Delete", c, handler.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}
	if dbUser == nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}
	if ok, _ := auth.ValidateOrgMembership(dbUser, org); !ok {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}
	if !auth.ValidateUserRoles(dbUser, org, nil, auth.TenantAdminRole) {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Tenant Admin role with org", nil)
	}
	tenant, err := common.GetTenantForOrg(ctx, nil, handler.dbSession, org)
	if err != nil {
		if err == common.ErrOrgTenantNotFound {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Org does not have a Tenant associated", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Tenant for org", nil)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Invalid SPX Partition ID", nil)
	}
	partitionDAO := cdbm.NewSpxPartitionDAO(handler.dbSession)
	partition, err := partitionDAO.GetByID(ctx, nil, id, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find SPX Partition with specified ID", nil)
		}
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve SPX Partition", nil)
	}
	if partition.TenantID != tenant.ID {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "SPX Partition is not owned by current org's Tenant", nil)
	}
	if partition.IsMissingOnSite {
		return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Could not find SPX Partition with specified ID", nil)
	}
	stc, err := handler.scp.GetClientByID(partition.SiteID)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}
	apiErr := common.ExecuteCoreGRPC(ctx, stc, corev1.Forge_DeleteSpxPartition_FullMethodName, partition.ToDeletionRequestProto(), &corev1.SpxPartitionDeletionResult{}, partition.SiteID.String())
	if apiErr != nil && apiErr.Code != http.StatusNotFound {
		logAPIError(logger, apiErr, "failed to delete SPX Partition via Core proxy")
		return cutil.NewAPIErrorResponse(c, apiErr.Code, apiErr.Message, nil)
	}
	if err := cdb.WithTx(ctx, handler.dbSession, func(tx *cdb.Tx) error { return partitionDAO.Delete(ctx, tx, partition.ID) }); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Core deleted SPX Partition but REST persistence cleanup failed", nil)
	}
	return c.NoContent(http.StatusNoContent)
}
