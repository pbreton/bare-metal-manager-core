/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

use chrono::{DateTime, Utc};
use serde::Deserialize;
use sqlx::FromRow;
use uuid::Uuid;

use crate::DatabaseError;

/// Row from the operating_systems table. Supports all variants: ipxe, image, ipxe_os_definition.
#[derive(Debug, Clone, FromRow, Deserialize)]
pub struct OperatingSystemRow {
    pub id: Uuid,
    pub name: String,
    pub description: Option<String>,
    pub org: String,
    #[sqlx(rename = "type")]
    #[serde(rename = "type")]
    pub type_: String,
    pub status: String,
    pub is_active: bool,
    pub allow_override: bool,
    pub phone_home_enabled: bool,
    pub user_data: Option<String>,
    pub created: DateTime<Utc>,
    pub updated: DateTime<Utc>,
    pub deleted: Option<DateTime<Utc>>,
    pub ipxe_script: Option<String>,
    pub os_image_id: Option<Uuid>,
    pub ipxe_template_name: Option<String>,
    pub ipxe_parameters: Option<sqlx::types::Json<serde_json::Value>>,
    pub ipxe_artifacts: Option<sqlx::types::Json<serde_json::Value>>,
    pub ipxe_definition_hash: Option<String>,
}

pub async fn get(
    txn: impl sqlx::Executor<'_, Database = sqlx::Postgres>,
    id: Uuid,
) -> Result<OperatingSystemRow, DatabaseError> {
    let query = "SELECT id, name, description, org, type, status, is_active, allow_override,
        phone_home_enabled, user_data, created, updated, deleted,
        ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash
        FROM operating_systems WHERE id = $1 AND deleted IS NULL";
    sqlx::query_as::<_, OperatingSystemRow>(query)
        .bind(id)
        .fetch_one(txn)
        .await
        .map_err(|e| DatabaseError::query(query, e))
}

/// Fetches multiple operating systems by id. Missing ids are skipped (no error).
pub async fn get_many(
    txn: impl sqlx::Executor<'_, Database = sqlx::Postgres>,
    ids: &[Uuid],
) -> Result<Vec<OperatingSystemRow>, DatabaseError> {
    if ids.is_empty() {
        return Ok(Vec::new());
    }
    let query = "SELECT id, name, description, org, type, status, is_active, allow_override,
        phone_home_enabled, user_data, created, updated, deleted,
        ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash
        FROM operating_systems WHERE id = ANY($1) AND deleted IS NULL";
    sqlx::query_as::<_, OperatingSystemRow>(query)
        .bind(ids)
        .fetch_all(txn)
        .await
        .map_err(|e| DatabaseError::query(query, e))
}

#[cfg(test)]
mod tests {
    use uuid::Uuid;

    use super::*;

    #[crate::sqlx_test]
    async fn test_get_returns_err_for_missing_id(pool: sqlx::PgPool) {
        let mut txn = pool.begin().await.unwrap();
        let id = Uuid::nil();
        let err = get(&mut *txn, id).await.unwrap_err();
        assert!(matches!(err, crate::DatabaseError::Sqlx(_)));
    }

    #[crate::sqlx_test]
    async fn test_get_many_returns_empty_for_missing_ids(pool: sqlx::PgPool) {
        let mut txn = pool.begin().await.unwrap();
        let rows = get_many(&mut *txn, &[Uuid::nil()]).await.unwrap();
        assert!(rows.is_empty());
    }
}
