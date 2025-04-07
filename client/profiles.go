package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type CreateProfileOptions struct {
	Name            string `json:"name"`
	UeIPPool        string `json:"ue-ip-pool"`
	DNS             string `json:"dns"`
	Mtu             int32  `json:"mtu"`
	BitrateUplink   string `json:"bitrate-uplink"`
	BitrateDownlink string `json:"bitrate-downlink"`
	Var5qi          int32  `json:"var5qi"`
	PriorityLevel   int32  `json:"priority-level"`
}

type GetProfileOptions struct {
	Name string `json:"name"`
}

type DeleteProfileOptions struct {
	Name string `json:"name"`
}

type Profile struct {
	Name            string `json:"name"`
	UeIPPool        string `json:"ue-ip-pool"`
	DNS             string `json:"dns"`
	Mtu             int32  `json:"mtu"`
	BitrateUplink   string `json:"bitrate-uplink"`
	BitrateDownlink string `json:"bitrate-downlink"`
	Var5qi          int32  `json:"var5qi"`
	PriorityLevel   int32  `json:"priority-level"`
}

func (c *Client) CreateProfile(opts *CreateProfileOptions) error {
	payload := struct {
		Name            string `json:"name"`
		UeIPPool        string `json:"ue-ip-pool"`
		DNS             string `json:"dns"`
		Mtu             int32  `json:"mtu"`
		BitrateUplink   string `json:"bitrate-uplink"`
		BitrateDownlink string `json:"bitrate-downlink"`
		Var5qi          int32  `json:"var5qi"`
		PriorityLevel   int32  `json:"priority-level"`
	}{
		Name:            opts.Name,
		UeIPPool:        opts.UeIPPool,
		DNS:             opts.DNS,
		Mtu:             opts.Mtu,
		BitrateUplink:   opts.BitrateUplink,
		BitrateDownlink: opts.BitrateDownlink,
		Var5qi:          opts.Var5qi,
		PriorityLevel:   opts.PriorityLevel,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/profiles",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetProfile(opts *GetProfileOptions) (*Profile, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/profiles/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var profileResponse Profile

	err = resp.DecodeResult(&profileResponse)
	if err != nil {
		return nil, err
	}
	return &profileResponse, nil
}

func (c *Client) DeleteProfile(opts *DeleteProfileOptions) error {
	_, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/profiles/" + opts.Name,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListProfiles() ([]*Profile, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/profiles",
	})
	if err != nil {
		return nil, err
	}
	var profiles []*Profile
	err = resp.DecodeResult(&profiles)
	if err != nil {
		return nil, err
	}
	return profiles, nil
}
