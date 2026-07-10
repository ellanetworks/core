// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/client"
)

// LocationData is a view of the spec-shaped LocationData response (TS 29.572)
// from POST /api/beta/location. Ncgi is set for NR, Ecgi for E-UTRA.
type LocationData struct {
	LocationEstimate    *GeoArea      `json:"locationEstimate"`
	PositioningDataList []MethodUsage `json:"positioningDataList"`
	Ncgi                *Ncgi         `json:"ncgi"`
	Ecgi                *Ecgi         `json:"ecgi"`
}

type GeoArea struct {
	Shape       string    `json:"shape"`
	Point       *GeoPoint `json:"point"`
	Uncertainty *float64  `json:"uncertainty"`
}

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type MethodUsage struct {
	Method string `json:"method"`
	Mode   string `json:"mode"`
	Usage  string `json:"usage"`
}

type Plmn struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Ncgi struct {
	PlmnID   Plmn   `json:"plmnId"`
	NrCellID string `json:"nrCellId"`
}

type Ecgi struct {
	PlmnID      Plmn   `json:"plmnId"`
	EutraCellID string `json:"eutraCellId"`
}

// PositioningMethod returns the first reported positioning method, or "".
func PositioningMethod(d *LocationData) string {
	if len(d.PositioningDataList) == 0 {
		return ""
	}

	return d.PositioningDataList[0].Method
}

// GetLocation calls POST /api/beta/location for the given method and decodes the
// spec-shaped LocationData response.
func GetLocation(ctx context.Context, cl *client.Client, supi, method string) (*LocationData, error) {
	body, err := json.Marshal(map[string]string{
		"supi":         supi,
		"request_type": "immediate",
		"method":       method,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	resp, err := cl.Requester.Do(ctx, &client.RequestOptions{
		Type:    client.SyncRequest,
		Method:  http.MethodPost,
		Path:    "/api/beta/location",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return nil, fmt.Errorf("POST location request failed: %w", err)
	}

	var result LocationData
	if err := resp.DecodeResult(&result); err != nil {
		return nil, fmt.Errorf("decode location response: %w", err)
	}

	return &result, nil
}

// ProvisionCellPosition provisions an antenna coordinate for a cell (rat "nr" or
// "eutra") so Cell-ID / E-CID can anchor a location estimate.
func ProvisionCellPosition(ctx context.Context, cl *client.Client, rat, mcc, mnc, cellID string) error {
	body, err := json.Marshal(map[string]any{
		"rat":                    rat,
		"mcc":                    mcc,
		"mnc":                    mnc,
		"cell_identity":          cellID,
		"latitude":               45.0,
		"longitude":              21.45,
		"uncertainty_semi_major": 150.0,
		"uncertainty_semi_minor": 150.0,
	})
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	_, err = cl.Requester.Do(ctx, &client.RequestOptions{
		Type:    client.SyncRequest,
		Method:  http.MethodPost,
		Path:    "/api/beta/cell-positions",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("POST cell-positions request failed: %w", err)
	}

	return nil
}
