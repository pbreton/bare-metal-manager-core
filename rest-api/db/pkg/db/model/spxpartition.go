// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/NVIDIA/infra-controller/rest-api/db/pkg/db"
	"github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/paginator"
	corev1 "github.com/NVIDIA/infra-controller/rest-api/proto/core/gen/v1"
)

const (
	SpxPartitionRelationName   = "SpxPartition"
	SpxPartitionOrderByDefault = "created"
)

var (
	SpxPartitionOrderByFields   = []string{"name", "vni", "created", "updated"}
	SpxPartitionRelatedEntities = map[string]bool{
		SiteRelationName:   true,
		TenantRelationName: true,
	}
)

// SpxPartition is the cloud projection of a Core SPX partition. ID and VNI are
// allocated by, or explicitly accepted by, Core and are therefore not separate
// controller-side lifecycle state.
type SpxPartition struct {
	bun.BaseModel `bun:"table:spx_partition,alias:spxp"`

	ID              uuid.UUID  `bun:"type:uuid,pk"`
	Name            string     `bun:"name,notnull"`
	Description     *string    `bun:"description"`
	Org             string     `bun:"org,notnull"`
	SiteID          uuid.UUID  `bun:"site_id,type:uuid,notnull"`
	Site            *Site      `bun:"rel:belongs-to,join:site_id=id"`
	TenantID        uuid.UUID  `bun:"tenant_id,type:uuid,notnull"`
	Tenant          *Tenant    `bun:"rel:belongs-to,join:tenant_id=id"`
	VNI             uint32     `bun:"vni,type:bigint,notnull"`
	Labels          Labels     `bun:"labels,type:jsonb,notnull"`
	IsMissingOnSite bool       `bun:"is_missing_on_site,notnull"`
	Created         time.Time  `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated         time.Time  `bun:"updated,nullzero,notnull,default:current_timestamp"`
	Deleted         *time.Time `bun:"deleted,soft_delete"`
	CreatedBy       uuid.UUID  `bun:"created_by,type:uuid,notnull"`
}

func (spxp *SpxPartition) Validate() error {
	return validation.ValidateStruct(spxp,
		validation.Field(&spxp.Name,
			validation.Required.Error("SPX Partition Name must be specified"),
			validation.Length(2, 256).Error("SPX Partition Name must be at least 2 characters and maximum 256 characters"),
			validation.By(validateSpxPartitionNameWhitespace)),
	)
}

func validateSpxPartitionNameWhitespace(value interface{}) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("SPX Partition Name must be a string")
	}
	if strings.TrimSpace(s) != s {
		return errors.New("SPX Partition Name must not contain leading or trailing whitespace")
	}
	return nil
}

func (spxp *SpxPartition) ToProto() *corev1.SpxPartition {
	description := ""
	if spxp.Description != nil {
		description = *spxp.Description
	}
	return &corev1.SpxPartition{
		Metadata:             &corev1.Metadata{Name: spxp.Name, Description: description, Labels: spxp.Labels.ToProto()},
		Id:                   &corev1.SpxPartitionId{Value: spxp.ID.String()},
		Vni:                  spxp.VNI,
		TenantOrganizationId: spxp.Org,
	}
}

func (spxp *SpxPartition) FromProto(partition *corev1.SpxPartition) {
	if partition == nil {
		return
	}
	if partition.Id != nil {
		if id, err := uuid.Parse(partition.Id.Value); err == nil {
			spxp.ID = id
		}
	}
	spxp.Name = ""
	spxp.Description = nil
	spxp.Labels = Labels{}
	if partition.Metadata != nil {
		spxp.Name = partition.Metadata.Name
		if partition.Metadata.Description != "" {
			description := partition.Metadata.Description
			spxp.Description = &description
		}
		spxp.Labels.FromProto(partition.Metadata.Labels)
	}
	spxp.VNI = partition.Vni
	spxp.Org = partition.TenantOrganizationId
}

func (spxp *SpxPartition) ToDeletionRequestProto() *corev1.SpxPartitionDeletionRequest {
	return &corev1.SpxPartitionDeletionRequest{Id: &corev1.SpxPartitionId{Value: spxp.ID.String()}}
}

type SpxPartitionCreateInput struct {
	SpxPartitionID uuid.UUID
	Name           string
	Description    *string
	TenantOrg      string
	SiteID         uuid.UUID
	TenantID       uuid.UUID
	VNI            uint32
	Labels         Labels
	CreatedBy      uuid.UUID
}

type SpxPartitionUpdateInput struct {
	SpxPartitionID  uuid.UUID
	Name            *string
	Description     **string
	TenantOrg       *string
	TenantID        *uuid.UUID
	VNI             *uint32
	IsMissingOnSite *bool
	Touch           bool
}

type SpxPartitionFilterInput struct {
	SpxPartitionIDs      []uuid.UUID
	Names                []string
	SiteIDs              []uuid.UUID
	TenantOrgs           []string
	TenantIDs            []uuid.UUID
	VNIs                 []uint32
	SearchQuery          *string
	IncludeMissingOnSite bool
}

var _ bun.BeforeAppendModelHook = (*SpxPartition)(nil)

func (spxp *SpxPartition) BeforeAppendModel(_ context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		spxp.Created = db.GetCurTime()
		spxp.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		spxp.Updated = db.GetCurTime()
	}
	return nil
}

var _ bun.BeforeCreateTableHook = (*SpxPartition)(nil)

func (spxp *SpxPartition) BeforeCreateTable(_ context.Context, query *bun.CreateTableQuery) error {
	query.ForeignKey(`("tenant_id") REFERENCES "tenant" ("id")`).
		ForeignKey(`("site_id") REFERENCES "site" ("id")`)
	return nil
}

type SpxPartitionDAO interface {
	GetByID(context.Context, *db.Tx, uuid.UUID, []string) (*SpxPartition, error)
	GetAll(context.Context, *db.Tx, SpxPartitionFilterInput, paginator.PageInput, []string) ([]SpxPartition, int, error)
	Create(context.Context, *db.Tx, SpxPartitionCreateInput) (*SpxPartition, error)
	Update(context.Context, *db.Tx, SpxPartitionUpdateInput) (*SpxPartition, error)
	Delete(context.Context, *db.Tx, uuid.UUID) error
}

type SpxPartitionSQLDAO struct {
	dbSession *db.Session
}

func (dao SpxPartitionSQLDAO) GetByID(ctx context.Context, tx *db.Tx, id uuid.UUID, includeRelations []string) (*SpxPartition, error) {
	partition := &SpxPartition{}
	query := db.GetIDB(tx, dao.dbSession).NewSelect().Model(partition).Where("spxp.id = ?", id)
	for _, relation := range includeRelations {
		query = query.Relation(relation)
	}
	if err := query.Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}
	return partition, nil
}

func (dao SpxPartitionSQLDAO) GetAll(ctx context.Context, tx *db.Tx, filter SpxPartitionFilterInput, page paginator.PageInput, includeRelations []string) ([]SpxPartition, int, error) {
	partitions := []SpxPartition{}
	query := db.GetIDB(tx, dao.dbSession).NewSelect().Model(&partitions)
	if filter.SpxPartitionIDs != nil {
		query = query.Where("spxp.id IN (?)", bun.In(filter.SpxPartitionIDs))
	}
	if filter.Names != nil {
		query = query.Where("spxp.name IN (?)", bun.In(filter.Names))
	}
	if filter.SiteIDs != nil {
		query = query.Where("spxp.site_id IN (?)", bun.In(filter.SiteIDs))
	}
	if filter.TenantOrgs != nil {
		query = query.Where("spxp.org IN (?)", bun.In(filter.TenantOrgs))
	}
	if filter.TenantIDs != nil {
		query = query.Where("spxp.tenant_id IN (?)", bun.In(filter.TenantIDs))
	}
	if filter.VNIs != nil {
		query = query.Where("spxp.vni IN (?)", bun.In(filter.VNIs))
	}
	if !filter.IncludeMissingOnSite {
		query = query.Where("spxp.is_missing_on_site = FALSE")
	}
	searchQuery, searchTokens, ok := db.NormalizeSearchQuery(filter.SearchQuery)
	if ok {
		query = query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("to_tsvector('english', coalesce(spxp.name, '') || ' ' || coalesce(spxp.description, '') || ' ' || coalesce(spxp.labels::text, '')) @@ to_tsquery('english', ?)", *searchTokens).
				WhereOr("spxp.name ILIKE ?", "%"+searchQuery+"%").
				WhereOr("spxp.description ILIKE ?", "%"+searchQuery+"%")
		})
	}
	for _, relation := range includeRelations {
		query = query.Relation(relation)
	}
	orderBy := page.OrderBy
	if orderBy == nil {
		orderBy = paginator.NewDefaultOrderBy(SpxPartitionOrderByDefault)
	}
	orders := []*paginator.OrderBy{orderBy}
	if orderBy.Field != "id" {
		orders = append(orders, &paginator.OrderBy{Field: "id", Order: orderBy.Order})
	}
	pageResult, err := paginator.NewPaginatorMultiOrderBy(ctx, query, page.Offset, page.Limit, orders, append(SpxPartitionOrderByFields, "id"))
	if err != nil {
		return nil, 0, err
	}
	if err := pageResult.Query.Limit(pageResult.Limit).Offset(pageResult.Offset).Scan(ctx); err != nil {
		return nil, 0, err
	}
	return partitions, pageResult.Total, nil
}

func (dao SpxPartitionSQLDAO) Create(ctx context.Context, tx *db.Tx, input SpxPartitionCreateInput) (*SpxPartition, error) {
	partition := &SpxPartition{
		ID: input.SpxPartitionID, Name: input.Name, Description: input.Description,
		Org: input.TenantOrg, SiteID: input.SiteID, TenantID: input.TenantID,
		VNI: input.VNI, Labels: input.Labels, CreatedBy: input.CreatedBy,
	}
	if partition.Labels == nil {
		partition.Labels = Labels{}
	}
	if err := partition.Validate(); err != nil {
		return nil, err
	}
	if _, err := db.GetIDB(tx, dao.dbSession).NewInsert().Model(partition).Exec(ctx); err != nil {
		return nil, err
	}
	return dao.GetByID(ctx, tx, partition.ID, nil)
}

func (dao SpxPartitionSQLDAO) Update(ctx context.Context, tx *db.Tx, input SpxPartitionUpdateInput) (*SpxPartition, error) {
	partition := &SpxPartition{ID: input.SpxPartitionID}
	columns := []string{}
	if input.Name != nil {
		partition.Name = *input.Name
		columns = append(columns, "name")
	}
	if input.Description != nil {
		partition.Description = *input.Description
		columns = append(columns, "description")
	}
	if input.TenantOrg != nil {
		partition.Org = *input.TenantOrg
		columns = append(columns, "org")
	}
	if input.TenantID != nil {
		partition.TenantID = *input.TenantID
		columns = append(columns, "tenant_id")
	}
	if input.VNI != nil {
		partition.VNI = *input.VNI
		columns = append(columns, "vni")
	}
	if input.IsMissingOnSite != nil {
		partition.IsMissingOnSite = *input.IsMissingOnSite
		columns = append(columns, "is_missing_on_site")
	}
	if len(columns) > 0 || input.Touch {
		columns = append(columns, "updated")
		if _, err := db.GetIDB(tx, dao.dbSession).NewUpdate().Model(partition).Column(columns...).Where("id = ?", partition.ID).Exec(ctx); err != nil {
			return nil, err
		}
	}
	return dao.GetByID(ctx, tx, partition.ID, nil)
}

func (dao SpxPartitionSQLDAO) Delete(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	_, err := db.GetIDB(tx, dao.dbSession).NewDelete().Model(&SpxPartition{ID: id}).Where("id = ?", id).Exec(ctx)
	return err
}

func NewSpxPartitionDAO(dbSession *db.Session) SpxPartitionDAO {
	return &SpxPartitionSQLDAO{dbSession: dbSession}
}
