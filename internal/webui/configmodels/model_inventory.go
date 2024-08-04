// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package configmodels

type Gnb struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type Upf struct {
	Hostname string `json:"hostname"`
	Port     string `json:"port"`
}
