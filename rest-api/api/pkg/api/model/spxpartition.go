// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	validationis "github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/google/uuid"

	"github.com/NVIDIA/infra-controller/rest-api/api/pkg/api/model/util"
	cdbm "github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
)

// APISpxPartitionCreateRequest is the caller-controlled SPX partition intent.
// Core remains responsible for accepting or allocating the VNI.
type APISpxPartitionCreateRequest struct {
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	SiteID      string            `json:"siteId"`
	VNI         *uint32           `json:"vni"`
	Labels      map[string]string `json:"labels"`
}

func (request *APISpxPartitionCreateRequest) Validate() error {
	if err := validation.ValidateStruct(request,
		validation.Field(&request.Name,
			validation.Required.Error(validationErrorStringLength),
			validation.By(util.ValidateNameCharacters),
			validation.Length(2, 256).Error(validationErrorStringLength)),
		validation.Field(&request.SiteID,
			validation.Required.Error(validationErrorValueRequired),
			validationis.UUID.Error(validationErrorInvalidUUID)),
	); err != nil {
		return err
	}
	return util.ValidateLabels(request.Labels)
}

func (request *APISpxPartitionCreateRequest) ToProto(id uuid.UUID, tenantOrg string) *corev1.SpxPartitionCreationRequest {
	labels := cdbm.Labels(request.Labels)
	description := ""
	if request.Description != nil {
		description = *request.Description
	}
	return &corev1.SpxPartitionCreationRequest{
		Metadata:             &corev1.Metadata{Name: request.Name, Description: description, Labels: labels.ToProto()},
		Id:                   &corev1.SpxPartitionId{Value: id.String()},
		Vni:                  request.VNI,
		TenantOrganizationId: tenantOrg,
	}
}

// APISpxPartition is the REST representation of a Core SPX partition.
type APISpxPartition struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	SiteID      string            `json:"siteId"`
	Site        *APISiteSummary   `json:"site"`
	TenantID    string            `json:"tenantId"`
	Tenant      *APITenantSummary `json:"tenant"`
	VNI         uint32            `json:"vni"`
	Labels      map[string]string `json:"labels"`
	Created     time.Time         `json:"created"`
	Updated     time.Time         `json:"updated"`
}

func NewAPISpxPartition(partition *cdbm.SpxPartition) *APISpxPartition {
	labels := map[string]string{}
	for key, value := range partition.Labels {
		labels[key] = value
	}
	response := &APISpxPartition{
		ID: partition.ID.String(), Name: partition.Name, Description: partition.Description,
		SiteID: partition.SiteID.String(), TenantID: partition.TenantID.String(),
		VNI: partition.VNI, Labels: labels, Created: partition.Created, Updated: partition.Updated,
	}
	if partition.Site != nil {
		response.Site = NewAPISiteSummary(partition.Site)
	}
	if partition.Tenant != nil {
		response.Tenant = NewAPITenantSummary(partition.Tenant)
	}
	return response
}
