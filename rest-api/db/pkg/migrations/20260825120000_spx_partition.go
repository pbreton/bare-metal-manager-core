// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/NVIDIA/infra-controller/rest-api/db/pkg/db/model"
)

func init() {
	Migrations.MustRegister(spxPartitionUpMigration, spxPartitionDownMigration)
}

func spxPartitionUpMigration(ctx context.Context, database *bun.DB) error {
	err := database.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewCreateTable().Model((*model.SpxPartition)(nil)).Exec(ctx); err != nil {
			return err
		}
		statements := []string{
			`CREATE INDEX spx_partition_site_id_idx ON spx_partition (site_id)`,
			`CREATE INDEX spx_partition_tenant_id_idx ON spx_partition (tenant_id)`,
			`CREATE INDEX spx_partition_created_idx ON spx_partition (created)`,
			`CREATE INDEX spx_partition_updated_idx ON spx_partition (updated)`,
			`CREATE INDEX spx_partition_tsv_idx ON spx_partition USING gin(to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, '') || ' ' || coalesce(labels::text, '')))`,
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Print(" [up migration] Created 'spx_partition' table. ")
	return nil
}

func spxPartitionDownMigration(ctx context.Context, database *bun.DB) error {
	_, err := database.NewDropTable().Model((*model.SpxPartition)(nil)).Exec(ctx)
	return err
}
