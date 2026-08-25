// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	tmocks "go.temporal.io/sdk/mocks"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/handler/util/common"
	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/model"
	sc "github.com/NVIDIA/infra-controller/rest-api/api/pkg/client/site"
	authz "github.com/NVIDIA/infra-controller/rest-api/auth/pkg/authorization"
	"github.com/NVIDIA/infra-controller/rest-api/common/pkg/grpcproxy"
	cutil "github.com/NVIDIA/infra-controller/rest-api/common/pkg/util"
	cdb "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db"
	cdbm "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
)

func TestSpxPartitionHandlersProxyCreateAndDeleteToCore(t *testing.T) {
	dbSession := common.TestInitDB(t)
	t.Cleanup(dbSession.Close)
	common.TestSetupSchema(t, dbSession)

	providerUser := common.TestBuildUser(t, dbSession, "provider-user", "provider-org", []string{authz.ProviderAdminRole})
	provider := common.TestBuildInfrastructureProvider(t, dbSession, "provider", "provider-org", providerUser)
	site := common.TestBuildSite(t, dbSession, provider, "site", providerUser)
	_, err := cdbm.NewSiteDAO(dbSession).Update(context.Background(), nil, cdbm.SiteUpdateInput{SiteID: site.ID, Status: cutil.GetPtr(cdbm.SiteStatusRegistered)})
	require.NoError(t, err)

	tenantOrg := "tenant-org"
	tenantUser := common.TestBuildUser(t, dbSession, "tenant-user", tenantOrg, []string{authz.TenantAdminRole})
	tenant := common.TestBuildTenant(t, dbSession, "tenant", tenantOrg, tenantUser)
	common.TestBuildTenantSite(t, dbSession, tenant, site, tenantUser)

	proxiedRequests := []grpcproxy.Request{}
	workflowRun := &tmocks.WorkflowRun{}
	workflowRun.On("Get", mock.Anything, mock.Anything).Run(func(arguments mock.Arguments) {
		response := arguments.Get(1).(*grpcproxy.Response)
		proxiedRequest := proxiedRequests[len(proxiedRequests)-1]
		var protoResponse []byte
		if proxiedRequest.FullMethod == corev1.Forge_CreateSpxPartition_FullMethodName {
			var createRequest corev1.SpxPartitionCreationRequest
			require.NoError(t, protojson.Unmarshal(proxiedRequest.RequestJSON, &createRequest))
			protoResponse, err = protojson.Marshal(&corev1.SpxPartition{
				Id: createRequest.Id,
				Metadata: &corev1.Metadata{
					Name: "core-canonical-name", Description: "Core canonical description",
				},
				Vni:                  4242,
				TenantOrganizationId: createRequest.TenantOrganizationId,
			})
		} else {
			protoResponse, err = protojson.Marshal(&corev1.SpxPartitionDeletionResult{})
		}
		require.NoError(t, err)
		response.ResponseJSON = protoResponse
	}).Return(nil)
	temporalClient := &tmocks.Client{}
	temporalClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, grpcproxy.Core.WorkflowName, mock.MatchedBy(func(request grpcproxy.Request) bool {
		proxiedRequests = append(proxiedRequests, request)
		return true
	})).Return(workflowRun, nil)
	clientPool := sc.NewClientPool(nil)
	clientPool.IDClientMap[site.ID.String()] = temporalClient

	requestedVNI := uint32(1234)
	createRequest := model.APISpxPartitionCreateRequest{
		Name: "spx-partition", Description: cutil.GetPtr("test partition"), SiteID: site.ID.String(),
		VNI: &requestedVNI, Labels: map[string]string{"environment": "test"},
	}
	body, err := json.Marshal(createRequest)
	require.NoError(t, err)
	createRecorder := invokeSpxPartitionHandler(t, NewCreateSpxPartitionHandler(dbSession, clientPool).Handle, tenantOrg, "", tenantUser, http.MethodPost, string(body))
	require.Equal(t, http.StatusCreated, createRecorder.Code, createRecorder.Body.String())

	var created model.APISpxPartition
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &created))
	assert.Equal(t, "core-canonical-name", created.Name)
	assert.Equal(t, "Core canonical description", *created.Description)
	assert.Equal(t, uint32(4242), created.VNI)
	assert.Equal(t, createRequest.Labels, created.Labels)
	require.Len(t, proxiedRequests, 1)
	assert.Equal(t, corev1.Forge_CreateSpxPartition_FullMethodName, proxiedRequests[0].FullMethod)
	var proxiedCreate corev1.SpxPartitionCreationRequest
	require.NoError(t, protojson.Unmarshal(proxiedRequests[0].RequestJSON, &proxiedCreate))
	assert.Equal(t, tenantOrg, proxiedCreate.TenantOrganizationId)
	assert.Equal(t, requestedVNI, proxiedCreate.GetVni())
	assert.Equal(t, createRequest.Name, proxiedCreate.Metadata.Name)
	assert.Equal(t, created.ID, proxiedCreate.Id.Value)

	getRecorder := invokeSpxPartitionHandler(t, NewGetSpxPartitionHandler(dbSession).Handle, tenantOrg, created.ID, tenantUser, http.MethodGet, "")
	require.Equal(t, http.StatusOK, getRecorder.Code, getRecorder.Body.String())
	var fetched model.APISpxPartition
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &fetched))
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Name, fetched.Name)
	assert.Equal(t, created.Description, fetched.Description)
	assert.Equal(t, created.SiteID, fetched.SiteID)
	assert.Equal(t, created.TenantID, fetched.TenantID)
	assert.Equal(t, created.VNI, fetched.VNI)
	assert.Equal(t, created.Labels, fetched.Labels)
	assert.Nil(t, fetched.Site)
	assert.Nil(t, fetched.Tenant)

	listRecorder := invokeSpxPartitionHandler(t, NewGetAllSpxPartitionHandler(dbSession).Handle, tenantOrg, "", tenantUser, http.MethodGet, "")
	require.Equal(t, http.StatusOK, listRecorder.Code, listRecorder.Body.String())
	var listed []model.APISpxPartition
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	deleteRecorder := invokeSpxPartitionHandler(t, NewDeleteSpxPartitionHandler(dbSession, clientPool).Handle, tenantOrg, created.ID, tenantUser, http.MethodDelete, "")
	require.Equal(t, http.StatusNoContent, deleteRecorder.Code, deleteRecorder.Body.String())
	require.Len(t, proxiedRequests, 2)
	assert.Equal(t, corev1.Forge_DeleteSpxPartition_FullMethodName, proxiedRequests[1].FullMethod)
	var proxiedDelete corev1.SpxPartitionDeletionRequest
	require.NoError(t, protojson.Unmarshal(proxiedRequests[1].RequestJSON, &proxiedDelete))
	assert.Equal(t, created.ID, proxiedDelete.Id.Value)
	_, err = cdbm.NewSpxPartitionDAO(dbSession).GetByID(context.Background(), nil, mustParseUUID(t, created.ID), nil)
	assert.ErrorIs(t, err, cdb.ErrDoesNotExist)
}

func invokeSpxPartitionHandler(t *testing.T, handler echo.HandlerFunc, org, id string, user *cdbm.User, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	if body != "" {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(request, recorder)
	ctx.SetParamNames("orgName", "id")
	ctx.SetParamValues(org, id)
	ctx.Set("user", user)
	require.NoError(t, handler(ctx))
	return recorder
}

func mustParseUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	require.NoError(t, err)
	return id
}
