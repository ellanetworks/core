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

type OperatorSlice struct {
	Sst int32  `json:"sst"`
	Sd  []byte `json:"sd"`
}

type OperatorID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type OperatorHomeNetwork struct {
	PrivateKey string `json:"private_key"`
}

type Operator struct {
	ID           OperatorID          `json:"id"`
	Slice        OperatorSlice       `json:"slice"`
	OperatorCode string              `json:"operator_code"`
	Tracking     OperatorTracking    `json:"tracking"`
	HomeNetwork  OperatorHomeNetwork `json:"home_network"`
}

type DataNetwork struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	IPPool string `json:"ip_pool"`
	DNS    string `json:"dns"`
	MTU    int32  `json:"mtu"`
}

type Policy struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	BitrateUplink   string `json:"bitrate_uplink"`
	BitrateDownlink string `json:"bitrate_downlink"`
	Var5qi          int32  `json:"var5qi"`
	Arp             int32  `json:"arp"`
	DataNetworkID   int    `json:"data_network_id"`
}

type Subscriber struct {
	ID             int     `json:"id"`
	Imsi           string  `json:"imsi"`
	IPAddress      *string `json:"ip_address"`
	SequenceNumber string  `json:"sequence_number"`
	PermanentKey   string  `json:"permanent_key"`
	Opc            string  `json:"opc"`
	PolicyID       int     `json:"policy_id"`
}

type Route struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
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
	DataNetworks      []DataNetwork `json:"data_networks"`
	Routes            []Route       `json:"routes"`
	NAT               bool          `json:"nat"`
	FlowAccounting    bool          `json:"flow_accounting"`
	N3ExternalAddress string        `json:"n3_external_address"`
}

type EllaCoreConfig struct {
	Operator    Operator     `json:"operator"`
	Networking  Networking   `json:"networking"`
	Policies    []Policy     `json:"policies"`
	Subscribers []Subscriber `json:"subscribers"`
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
	Imsi       string `json:"imsi"`
	Registered bool   `json:"registered"`
	IPAddress  string `json:"ip_address"`
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

func (fc *Fleet) Register(ctx context.Context, activationToken string, publicKey ecdsa.PublicKey, initialConfig EllaCoreConfig, initialStatus EllaCoreStatus, initialMetrics EllaCoreMetrics, initialUsage []SubscriberUsageEntry) (*RegisterResponse, error) {
	pubKeyPEM, err := marshalPublicKey(&publicKey)
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal public key: %w", err)
	}

	params := &RegisterParams{
		ActivationToken: activationToken,
		PublicKey:       pubKeyPEM,
		InitialConfig:   initialConfig,
		InitialStatus:   initialStatus,
		InitialMetrics:  initialMetrics,
		InitialUsage:    initialUsage,
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
