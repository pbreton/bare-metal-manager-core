// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package spxpartition

func (api *API) Init() {
	ManagerAccess.Data.EB.Log.Info().Msg("SpxPartition: Initializing SPX Partition API")
}

func (api *API) GetState() []string { return nil }
