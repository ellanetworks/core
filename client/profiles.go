package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateProfileOptions contains the parameters for creating a new profile.
type CreateProfileOptions struct {
	Name string `json:"name"`
	// UeAmbrUplink is the aggregate uplink bitrate cap across all of the subscriber's
	// sessions (UE-AMBR). Enforced by the radio. Example: "1 Gbps".
	UeAmbrUplink string `json:"ue_ambr_uplink"`
	// UeAmbrDownlink is the aggregate downlink bitrate cap across all of the subscriber's
	// sessions (UE-AMBR). Enforced by the radio. Example: "1 Gbps".
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type UpdateProfileOptions struct {
	UeAmbrUplink   string `json:"ue_ambr_uplink,omitempty"`
	UeAmbrDownlink string `json:"ue_ambr_downlink,omitempty"`
}

type GetProfileOptions struct {
	Name string `json:"name"`
}

type DeleteProfileOptions struct {
	Name string `json:"name"`
}

// Profile represents a subscriber profile with UE-AMBR settings.
// UE-AMBR caps aggregate non-GBR throughput across all of a subscriber's PDU sessions
// and is enforced by the radio.
type Profile struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type ListProfilesResponse struct {
	Items      []Profile `json:"items"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	TotalCount int       `json:"total_count"`
}

// CreateProfile creates a new profile.
func (c *Client) CreateProfile(ctx context.Context, opts *CreateProfileOptions) error {
	payload := struct {
		Name           string `json:"name"`
		UeAmbrUplink   string `json:"ue_ambr_uplink"`
		UeAmbrDownlink string `json:"ue_ambr_downlink"`
	}{
		Name:           opts.Name,
		UeAmbrUplink:   opts.UeAmbrUplink,
		UeAmbrDownlink: opts.UeAmbrDownlink,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
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

// GetProfile retrieves a profile by name.
func (c *Client) GetProfile(ctx context.Context, opts *GetProfileOptions) (*Profile, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/profiles/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var profile Profile

	err = resp.DecodeResult(&profile)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

// UpdateProfile updates an existing profile by name.
func (c *Client) UpdateProfile(ctx context.Context, name string, opts *UpdateProfileOptions) error {
	payload := struct {
		UeAmbrUplink   string `json:"ue_ambr_uplink,omitempty"`
		UeAmbrDownlink string `json:"ue_ambr_downlink,omitempty"`
	}{
		UeAmbrUplink:   opts.UeAmbrUplink,
		UeAmbrDownlink: opts.UeAmbrDownlink,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/profiles/" + name,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteProfile deletes a profile by name.
func (c *Client) DeleteProfile(ctx context.Context, opts *DeleteProfileOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/profiles/" + opts.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListProfiles lists profiles with pagination.
func (c *Client) ListProfiles(ctx context.Context, p *ListParams) (*ListProfilesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/profiles",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var profiles ListProfilesResponse

	err = resp.DecodeResult(&profiles)
	if err != nil {
		return nil, err
	}

	return &profiles, nil
}
