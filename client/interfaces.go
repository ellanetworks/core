package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type InterfaceVlan struct {
	MasterInterface string `json:"master_interface"`
	VlanId          int    `json:"vlan_id"`
}

type N2Interface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Interface string   `json:"interface,omitempty"`
}

type N3Interface struct {
	Name            string         `json:"name"`
	Addresses       []string       `json:"addresses"`
	ExternalAddress string         `json:"external_address"`
	Vlan            *InterfaceVlan `json:"vlan,omitempty"`
}

type N6Interface struct {
	Name      string         `json:"name"`
	Addresses []string       `json:"addresses"`
	Vlan      *InterfaceVlan `json:"vlan,omitempty"`
}

type APIInterface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
}

type NetworkInterfaces struct {
	N2  N2Interface  `json:"n2"`
	N3  N3Interface  `json:"n3"`
	N6  N6Interface  `json:"n6"`
	API APIInterface `json:"api"`
}

type UpdateN3InterfaceOptions struct {
	ExternalAddress string `json:"external_address"`
}

// ListNetworkInterfaces retrieves the current networking interface configuration
// (N2, N3, N6, and API), including resolved addresses and VLAN settings.
func (c *Client) ListNetworkInterfaces(ctx context.Context) (*NetworkInterfaces, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/interfaces",
	})
	if err != nil {
		return nil, err
	}

	var interfacesResponse NetworkInterfaces

	err = resp.DecodeResult(&interfacesResponse)
	if err != nil {
		return nil, err
	}

	return &interfacesResponse, nil
}

// UpdateN3Interface updates the N3 interface's external address. An empty string
// means "use the local interface IP"; the UPF reconciler resolves that against
// each node's local config when applying.
func (c *Client) UpdateN3Interface(ctx context.Context, opts *UpdateN3InterfaceOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/interfaces/n3",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
