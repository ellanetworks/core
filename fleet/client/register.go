// Copyright 2026 Ella Networks

package client

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
)

// ErrUnauthorized is returned when the fleet server rejects the activation token (HTTP 401).
var ErrUnauthorized = errors.New("invalid activation code")

type OperatorTracking struct {
	SupportedTacs []string `json:"supported_tacs"`
}

type OperatorNASSecurity struct {
	Ciphering []string `json:"ciphering"`
	Integrity []string `json:"integrity"`
}

type OperatorSPN struct {
	FullName  string `json:"full_name"`
	ShortName string `json:"short_name"`
}

type OperatorAMF struct {
	RegionID int `json:"region_id"`
	SetID    int `json:"set_id"`
}

type OperatorID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Operator struct {
	ID           OperatorID          `json:"id"`
	OperatorCode string              `json:"operator_code"`
	Tracking     OperatorTracking    `json:"tracking"`
	NASSecurity  OperatorNASSecurity `json:"nas_security"`
	SPN          OperatorSPN         `json:"spn"`
	AMF          OperatorAMF         `json:"amf"`
}

type HomeNetworkKey struct {
	KeyIdentifier int    `json:"key_identifier"`
	Scheme        string `json:"scheme"`
	PrivateKey    string `json:"private_key"`
}

type DataNetwork struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool"`
	DNS    string `json:"dns"`
	MTU    int32  `json:"mtu"`
}

type Profile struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type Slice struct {
	Name string  `json:"name"`
	Sst  int32   `json:"sst"`
	Sd   *string `json:"sd,omitempty"`
}

type Policy struct {
	Name                string `json:"name"`
	ProfileName         string `json:"profile_name"`
	SliceName           string `json:"slice_name"`
	DataNetworkName     string `json:"data_network_name"`
	Var5qi              int32  `json:"var5qi"`
	Arp                 int32  `json:"arp"`
	SessionAmbrUplink   string `json:"session_ambr_uplink"`
	SessionAmbrDownlink string `json:"session_ambr_downlink"`
}

type Subscriber struct {
	Imsi           string `json:"imsi"`
	ProfileName    string `json:"profile_name"`
	SequenceNumber string `json:"sequence_number"`
	PermanentKey   string `json:"permanent_key"`
	Opc            string `json:"opc"`
}

// NetworkRule is a per-policy filter rule. Identified by
// (policy_name, direction, precedence); precedence is 1-indexed and
// unique within (policy_name, direction).
type NetworkRule struct {
	PolicyName   string  `json:"policy_name"`
	Direction    string  `json:"direction"` // "uplink" or "downlink"
	Precedence   int32   `json:"precedence"`
	Description  string  `json:"description"`
	RemotePrefix *string `json:"remote_prefix,omitempty"`
	Protocol     int32   `json:"protocol"`
	PortLow      int32   `json:"port_low"`
	PortHigh     int32   `json:"port_high"`
	Action       string  `json:"action"` // "allow" or "deny"
}

type Route struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type BGPSettings struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"local_as"`
	RouterID      string `json:"router_id"`
	ListenAddress string `json:"listen_address"`
}

// BGPPeer is keyed by Address (unique per cluster). Fleet-pushed peers
// land with cluster-wide scope (nodeID=NULL on the Core); node-local
// BGP peers are not managed by Fleet.
type BGPPeer struct {
	Address     string `json:"address"`
	RemoteAS    int    `json:"remote_as"`
	HoldTime    int    `json:"hold_time"`
	Password    string `json:"password,omitempty"`
	Description string `json:"description"`
}

// BGPImportPrefix references a peer by its Address (natural key).
type BGPImportPrefix struct {
	PeerAddress string `json:"peer_address"`
	Prefix      string `json:"prefix"`
	MaxLength   int    `json:"max_length"`
}

// RetentionPolicy holds the retention days for a single category:
// "audit", "radio", "usage", or "flow_reports".
type RetentionPolicy struct {
	Category string `json:"category"`
	Days     int    `json:"days"`
}

type N2Interface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type Vlan struct {
	MasterInterface string `json:"master_interface"`
	VlanId          int    `json:"vlan_id"`
}

type N3Interface struct {
	Name            string `json:"name"`
	Address         string `json:"address"`
	ExternalAddress string `json:"external_address"`
	Vlan            *Vlan  `json:"vlan,omitempty"`
}

type N6Interface struct {
	Name string `json:"name"`
	Vlan *Vlan  `json:"vlan,omitempty"`
}

type APIInterface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type StatusNetworkInterfaces struct {
	N2  N2Interface  `json:"n2"`
	N3  N3Interface  `json:"n3"`
	N6  N6Interface  `json:"n6"`
	API APIInterface `json:"api"`
}

type Networking struct {
	DataNetworks      []DataNetwork     `json:"data_networks"`
	Routes            []Route           `json:"routes"`
	NAT               bool              `json:"nat"`
	FlowAccounting    bool              `json:"flow_accounting"`
	N3ExternalAddress string            `json:"n3_external_address"`
	BGP               BGPSettings       `json:"bgp"`
	BGPPeers          []BGPPeer         `json:"bgp_peers"`
	BGPImportPrefixes []BGPImportPrefix `json:"bgp_import_prefixes"`
}

type EllaCoreConfig struct {
	Operator          Operator          `json:"operator"`
	HomeNetworkKeys   []HomeNetworkKey  `json:"home_network_keys"`
	Networking        Networking        `json:"networking"`
	Profiles          []Profile         `json:"profiles"`
	Slices            []Slice           `json:"slices"`
	Policies          []Policy          `json:"policies"`
	NetworkRules      []NetworkRule     `json:"network_rules"`
	Subscribers       []Subscriber      `json:"subscribers"`
	RetentionPolicies []RetentionPolicy `json:"retention_policies"`
}

type PlmnID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnID PlmnID `json:"plmnID"`
	Tac    string `json:"tac"`
}

type Snssai struct {
	Sst int32  `json:"sst"`
	Sd  string `json:"sd"`
}

type SupportedTAI struct {
	Tai     Tai      `json:"tai"`
	SNssais []Snssai `json:"snssais"`
}

type Radio struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

type SubscriberStatus struct {
	Imsi               string `json:"imsi"`
	Registered         bool   `json:"registered"`
	IPAddress          string `json:"ip_address"`
	Imei               string `json:"imei,omitempty"`
	CipheringAlgorithm string `json:"ciphering_algorithm,omitempty"`
	IntegrityAlgorithm string `json:"integrity_algorithm,omitempty"`
	LastSeenAt         string `json:"last_seen_at,omitempty"`
	LastSeenRadio      string `json:"last_seen_radio,omitempty"`
}

type EllaCoreStatus struct {
	NetworkInterfaces StatusNetworkInterfaces `json:"network_interfaces"`
	Radios            []Radio                 `json:"radios"`
	Subscribers       []SubscriberStatus      `json:"subscribers"`
}

type EllaCoreMetrics struct {
	UplinkBytesTotal             int64   `json:"uplink_bytes_total"`
	DownlinkBytesTotal           int64   `json:"downlink_bytes_total"`
	PDUSessionsTotal             int64   `json:"pdu_sessions_total"`
	ProcessCPUSecondsTotal       float64 `json:"process_cpu_seconds_total"`
	ProcessResidentMemoryBytes   int64   `json:"process_resident_memory_bytes"`
	GoGoroutines                 int64   `json:"go_goroutines"`
	ProcessStartTimeSeconds      float64 `json:"process_start_time_seconds"`
	DatabaseStorageBytes         int64   `json:"database_storage_bytes"`
	IPAddresses                  int64   `json:"ip_addresses"`
	IPAddressesAllocated         int64   `json:"ip_addresses_allocated"`
	RegistrationAttemptsAccepted int64   `json:"registration_attempts_accepted"`
	RegistrationAttemptsRejected int64   `json:"registration_attempts_rejected"`
}

type RegisterParams struct {
	ActivationToken string                 `json:"activation_token"`
	PublicKey       string                 `json:"public_key"`
	ClusterID       string                 `json:"cluster_id,omitempty"`
	NodeID          int                    `json:"node_id,omitempty"`
	InitialConfig   EllaCoreConfig         `json:"initial_config"`
	InitialStatus   EllaCoreStatus         `json:"initial_status"`
	InitialMetrics  EllaCoreMetrics        `json:"initial_metrics"`
	InitialUsage    []SubscriberUsageEntry `json:"initial_usage,omitempty"`
}

type RegisterResponse struct {
	Certificate   string `json:"certificate"`
	CACertificate string `json:"ca_certificate"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Response struct {
	Result json.RawMessage `json:"result"`
}

type RegisterInput struct {
	ActivationToken string
	PublicKey       ecdsa.PublicKey
	ClusterID       string
	NodeID          int
	InitialConfig   EllaCoreConfig
	InitialStatus   EllaCoreStatus
	InitialMetrics  EllaCoreMetrics
	InitialUsage    []SubscriberUsageEntry
}

func (fc *Fleet) Register(ctx context.Context, in RegisterInput) (*RegisterResponse, error) {
	pubKeyPEM, err := marshalPublicKey(&in.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal public key: %w", err)
	}

	params := &RegisterParams{
		ActivationToken: in.ActivationToken,
		PublicKey:       pubKeyPEM,
		ClusterID:       in.ClusterID,
		NodeID:          in.NodeID,
		InitialConfig:   in.InitialConfig,
		InitialStatus:   in.InitialStatus,
		InitialMetrics:  in.InitialMetrics,
		InitialUsage:    in.InitialUsage,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fc.url+"/api/v1/cores/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := fc.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if err := checkResponseContentType(res); err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("unexpected status code %d and failed to decode error: %w", res.StatusCode, err)
		}

		if res.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, errResp.Error)
		}

		return nil, fmt.Errorf("register failed (status %d): %s", res.StatusCode, errResp.Error)
	}

	var envelope Response
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response envelope: %w", err)
	}

	var registerResponse RegisterResponse
	if err := json.Unmarshal(envelope.Result, &registerResponse); err != nil {
		return nil, fmt.Errorf("decoding register result: %w", err)
	}

	return &registerResponse, nil
}

func marshalPublicKey(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshalling public key: %w", err)
	}

	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}

	return string(pem.EncodeToMemory(block)), nil
}
