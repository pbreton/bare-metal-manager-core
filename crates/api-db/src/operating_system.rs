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
use sqlx::{FromRow, PgConnection};
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

pub async fn list(
    txn: &mut PgConnection,
    org: Option<&str>,
) -> Result<Vec<OperatingSystemRow>, DatabaseError> {
    if let Some(org) = org {
        let query = "SELECT id, name, description, org, type, status, is_active, allow_override,
            phone_home_enabled, user_data, created, updated, deleted,
            ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash
            FROM operating_systems WHERE org = $1 AND deleted IS NULL ORDER BY name";
        sqlx::query_as::<_, OperatingSystemRow>(query)
            .bind(org)
            .fetch_all(txn)
            .await
            .map_err(|e| DatabaseError::query(query, e))
    } else {
        let query = "SELECT id, name, description, org, type, status, is_active, allow_override,
            phone_home_enabled, user_data, created, updated, deleted,
            ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash
            FROM operating_systems WHERE deleted IS NULL ORDER BY name";
        sqlx::query_as::<_, OperatingSystemRow>(query)
            .fetch_all(txn)
            .await
            .map_err(|e| DatabaseError::query(query, e))
    }
}

#[derive(Debug)]
pub struct CreateOperatingSystem {
    pub name: String,
    pub description: Option<String>,
    pub org: String,
    pub type_: String,
    pub is_active: bool,
    pub allow_override: bool,
    pub phone_home_enabled: bool,
    pub user_data: Option<String>,
    pub ipxe_script: Option<String>,
    pub os_image_id: Option<Uuid>,
    pub ipxe_template_name: Option<String>,
    pub ipxe_parameters: Option<serde_json::Value>,
    pub ipxe_artifacts: Option<serde_json::Value>,
    pub ipxe_definition_hash: Option<String>,
}

pub async fn create(
    txn: &mut PgConnection,
    input: &CreateOperatingSystem,
) -> Result<OperatingSystemRow, DatabaseError> {
    let query = "INSERT INTO operating_systems
        (name, description, org, type, is_active, allow_override, phone_home_enabled, user_data,
         ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash)
        VALUES ($1, $2, $3, $4::operating_system_type, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
        RETURNING id, name, description, org, type, status, is_active, allow_override,
        phone_home_enabled, user_data, created, updated, deleted,
        ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash";
    sqlx::query_as::<_, OperatingSystemRow>(query)
        .bind(&input.name)
        .bind(&input.description)
        .bind(&input.org)
        .bind(&input.type_)
        .bind(input.is_active)
        .bind(input.allow_override)
        .bind(input.phone_home_enabled)
        .bind(&input.user_data)
        .bind(&input.ipxe_script)
        .bind(input.os_image_id)
        .bind(&input.ipxe_template_name)
        .bind(input.ipxe_parameters.as_ref().map(sqlx::types::Json))
        .bind(input.ipxe_artifacts.as_ref().map(sqlx::types::Json))
        .bind(&input.ipxe_definition_hash)
        .fetch_one(txn)
        .await
        .map_err(|e| DatabaseError::query(query, e))
}

#[derive(Debug)]
pub struct UpdateOperatingSystem {
    pub id: Uuid,
    pub name: Option<String>,
    pub description: Option<String>,
    pub is_active: Option<bool>,
    pub allow_override: Option<bool>,
    pub phone_home_enabled: Option<bool>,
    pub user_data: Option<String>,
    pub ipxe_script: Option<String>,
    pub ipxe_template_name: Option<String>,
    pub ipxe_parameters: Option<serde_json::Value>,
    pub ipxe_artifacts: Option<serde_json::Value>,
}

pub async fn update(
    txn: &mut PgConnection,
    existing: &OperatingSystemRow,
    input: &UpdateOperatingSystem,
) -> Result<OperatingSystemRow, DatabaseError> {
    let name = input.name.as_deref().unwrap_or(&existing.name);
    let description = input.description.as_deref().or(existing.description.as_deref());
    let is_active = input.is_active.unwrap_or(existing.is_active);
    let allow_override = input.allow_override.unwrap_or(existing.allow_override);
    let phone_home_enabled = input.phone_home_enabled.unwrap_or(existing.phone_home_enabled);
    let user_data = input.user_data.as_deref().or(existing.user_data.as_deref());
    let ipxe_script = input.ipxe_script.as_deref().or(existing.ipxe_script.as_deref());
    let ipxe_template_name = input.ipxe_template_name.as_deref().or(existing.ipxe_template_name.as_deref());

    let ipxe_parameters: Option<sqlx::types::Json<&serde_json::Value>> = input
        .ipxe_parameters
        .as_ref()
        .or(existing.ipxe_parameters.as_ref().map(|j| &j.0))
        .map(|v| sqlx::types::Json(v));
    let ipxe_artifacts: Option<sqlx::types::Json<&serde_json::Value>> = input
        .ipxe_artifacts
        .as_ref()
        .or(existing.ipxe_artifacts.as_ref().map(|j| &j.0))
        .map(|v| sqlx::types::Json(v));

    let query = "UPDATE operating_systems SET
        name = $1, description = $2, is_active = $3, allow_override = $4,
        phone_home_enabled = $5, user_data = $6, ipxe_script = $7,
        ipxe_template_name = $8, ipxe_parameters = $9, ipxe_artifacts = $10,
        updated = NOW()
        WHERE id = $11 AND deleted IS NULL
        RETURNING id, name, description, org, type, status, is_active, allow_override,
        phone_home_enabled, user_data, created, updated, deleted,
        ipxe_script, os_image_id, ipxe_template_name, ipxe_parameters, ipxe_artifacts, ipxe_definition_hash";
    sqlx::query_as::<_, OperatingSystemRow>(query)
        .bind(name)
        .bind(description)
        .bind(is_active)
        .bind(allow_override)
        .bind(phone_home_enabled)
        .bind(user_data)
        .bind(ipxe_script)
        .bind(ipxe_template_name)
        .bind(ipxe_parameters)
        .bind(ipxe_artifacts)
        .bind(input.id)
        .fetch_one(txn)
        .await
        .map_err(|e| DatabaseError::query(query, e))
}

pub async fn delete(
    txn: &mut PgConnection,
    id: Uuid,
) -> Result<(), DatabaseError> {
    let query = "UPDATE operating_systems SET deleted = NOW(), updated = NOW() WHERE id = $1 AND deleted IS NULL";
    let result = sqlx::query(query)
        .bind(id)
        .execute(txn)
        .await
        .map_err(|e| DatabaseError::query(query, e))?;
    if result.rows_affected() == 0 {
        return Err(DatabaseError::NotFoundError {
            kind: "OperatingSystem",
            id: id.to_string(),
        });
    }
    Ok(())
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
