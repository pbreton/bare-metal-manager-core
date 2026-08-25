// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package managerapi

type SpxPartitionInterface interface {
	Init()
	RegisterPublisher() error
	RegisterCron() error
	GetState() []string
}
