// Copyright 2024 Ella Networks

package config

type UpfConfig struct {
	InterfaceName []string `mapstructure:"interface_name" json:"interface_name"`
	XDPAttachMode string   `mapstructure:"xdp_attach_mode" validate:"oneof=generic native offload" json:"xdp_attach_mode"`
	ApiAddress    string   `mapstructure:"api_address" validate:"hostname_port" json:"api_address"`
	PfcpAddress   string   `mapstructure:"pfcp_address" validate:"hostname_port" json:"pfcp_address"`
	PfcpNodeId    string   `mapstructure:"pfcp_node_id" validate:"hostname|ip" json:"pfcp_node_id"`
	N3Address     string   `mapstructure:"n3_address" validate:"ipv4" json:"n3_address"`
	GtpPeer       []string `mapstructure:"gtp_peer" validate:"omitempty,dive,hostname_port" json:"gtp_peer"`
	EchoInterval  uint32   `mapstructure:"echo_interval" validate:"min=1" json:"echo_interval"`
	QerMapSize    uint32   `mapstructure:"qer_map_size" validate:"min=1" json:"qer_map_size"`
	FarMapSize    uint32   `mapstructure:"far_map_size" validate:"min=1" json:"far_map_size"`
	PdrMapSize    uint32   `mapstructure:"pdr_map_size" validate:"min=1" json:"pdr_map_size"`
	EbpfMapResize bool     `mapstructure:"resize_ebpf_maps" json:"resize_ebpf_maps"`
	LoggingLevel  string   `mapstructure:"logging_level" validate:"required" json:"logging_level"`
	FTEIDPool     uint32   `mapstructure:"teid_pool" json:"teid_pool"`
	FeatureUEIP   bool     `mapstructure:"feature_ueip" json:"feature_ueip"`
	FeatureFTUP   bool     `mapstructure:"feature_ftup" json:"feature_ftup"`
}
