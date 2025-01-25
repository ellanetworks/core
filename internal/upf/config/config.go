// Copyright 2024 Ella Networks

package config

type UpfConfig struct {
	InterfaceName []string
	XDPAttachMode string
	PfcpAddress   string
	PfcpNodeId    string
	SmfAddress    string
	SmfNodeId     string
	N3Address     string
	QerMapSize    uint32
	FarMapSize    uint32
	PdrMapSize    uint32
	FTEIDPool     uint32
	FeatureUEIP   bool
}
