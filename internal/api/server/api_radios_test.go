// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

const (
	RadioName = "gnb-001"
)

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
	Name           string         `json:"name"`
	ID             string         `json:"id"`
	Address        string         `json:"address"`
	RanNodeType    string         `json:"type"`
	Status         string         `json:"status"`
	ConnectedAt    string         `json:"connected_at"`
	LastSeenAt     string         `json:"last_seen_at"`
	DisconnectedAt string         `json:"disconnected_at"`
	SupportedTAIs  []SupportedTAI `json:"supported_tais"`
}

type GetRadioResponse struct {
	Result Radio  `json:"result"`
	Error  string `json:"error,omitempty"`
}

type ListRadiosResponseResult struct {
	Items      []Radio `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

type ListRadiosResponse struct {
	Result ListRadiosResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func listRadios(url string, client *http.Client, token string, page int, perPage int) (int, *ListRadiosResponse, error) {
	return apiDo[ListRadiosResponse](client, "GET", fmt.Sprintf("%s/api/v1/ran/radios?page=%d&per_page=%d", url, page, perPage), token, nil)
}

func TestListRadios(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	amfInstance := env.AMF
	ran1 := amf.Radio{}
	ran1.RanID = &models.GlobalRanNodeID{
		GNbID: &models.GNbID{
			GNBValue: "mcc:001:mnc:01:gnb-001",
		},
	}
	ran1.RanPresent = amf.RanPresentGNbID
	amfInstance.UpdateRadioName(&ran1, "gnb-001")
	amfInstance.UpdateRadioSupportedTAIs(&ran1, []amf.SupportedTAI{
		{
			Tai: models.Tai{
				PlmnID: &models.PlmnID{
					Mcc: "123",
					Mnc: "12",
				},
				Tac: "0002",
			},
			SNssaiList: []models.Snssai{
				{
					Sst: 2,
					Sd:  "010204",
				},
			},
		},
	})
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), &ran1)

	ran2 := amf.Radio{}
	ran2.RanID = &models.GlobalRanNodeID{
		GNbID: &models.GNbID{
			GNBValue: "mcc:001:mnc:01:gnb-002",
		},
	}
	ran2.RanPresent = amf.RanPresentGNbID
	amfInstance.UpdateRadioName(&ran2, "gnb-002")
	amfInstance.UpdateRadioSupportedTAIs(&ran2, []amf.SupportedTAI{
		{
			Tai: models.Tai{
				PlmnID: &models.PlmnID{
					Mcc: "001",
					Mnc: "01",
				},
				Tac: "0001",
			},
			SNssaiList: []models.Snssai{
				{
					Sst: 1,
					Sd:  "010203",
				},
			},
		},
	})
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), &ran2)

	// Set up the Gin router
	statusCode, response, err := listRadios(env.Server.URL, client, token, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list radios: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	if len(response.Result.Items) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(response.Result.Items))
	}

	for _, radio := range response.Result.Items {
		switch radio.Name {
		case "gnb-001":
			if radio.Address != "" {
				t.Fatalf("expected radio address %q, got %q", "", radio.Address)
			}

			if radio.ID != "mcc:001:mnc:01:gnb-001" {
				t.Fatalf("expected radio ID %q, got %q", "mcc:001:mnc:01:gnb-001", radio.ID)
			}

			if len(radio.SupportedTAIs) != 1 {
				t.Fatalf("expected 1 supported TAI, got %d", len(radio.SupportedTAIs))
			}

			if radio.SupportedTAIs[0].Tai.PlmnID.Mcc != "123" {
				t.Fatalf("expected mcc %q, got %q", "123", radio.SupportedTAIs[0].Tai.PlmnID.Mcc)
			}

			if radio.SupportedTAIs[0].Tai.PlmnID.Mnc != "12" {
				t.Fatalf("expected mnc %q, got %q", "12", radio.SupportedTAIs[0].Tai.PlmnID.Mnc)
			}

			if radio.SupportedTAIs[0].Tai.Tac != "0002" {
				t.Fatalf("expected tac %q, got %q", "0002", radio.SupportedTAIs[0].Tai.Tac)
			}

			if len(radio.SupportedTAIs[0].SNssais) != 1 {
				t.Fatalf("expected 1 supported SNssai, got %d", len(radio.SupportedTAIs[0].SNssais))
			}

			if radio.SupportedTAIs[0].SNssais[0].Sst != 2 {
				t.Fatalf("expected sst %d, got %d", 2, radio.SupportedTAIs[0].SNssais[0].Sst)
			}

			if radio.SupportedTAIs[0].SNssais[0].Sd != "010204" {
				t.Fatalf("expected sd %q, got %q", "010204", radio.SupportedTAIs[0].SNssais[0].Sd)
			}
		case "gnb-002":
			if radio.Address != "" {
				t.Fatalf("expected radio address %q, got %q", "", radio.Address)
			}

			if radio.ID != "mcc:001:mnc:01:gnb-002" {
				t.Fatalf("expected radio ID %q, got %q", "mcc:001:mnc:01:gnb-002", radio.ID)
			}

			if len(radio.SupportedTAIs) != 1 {
				t.Fatalf("expected 1 supported TAI, got %d", len(radio.SupportedTAIs))
			}

			if radio.SupportedTAIs[0].Tai.PlmnID.Mcc != "001" {
				t.Fatalf("expected mcc %q, got %q", "001", radio.SupportedTAIs[0].Tai.PlmnID.Mcc)
			}

			if radio.SupportedTAIs[0].Tai.PlmnID.Mnc != "01" {
				t.Fatalf("expected mnc %q, got %q", "01", radio.SupportedTAIs[0].Tai.PlmnID.Mnc)
			}

			if radio.SupportedTAIs[0].Tai.Tac != "0001" {
				t.Fatalf("expected tac %q, got %q", "0001", radio.SupportedTAIs[0].Tai.Tac)
			}

			if len(radio.SupportedTAIs[0].SNssais) != 1 {
				t.Fatalf("expected 1 supported SNssai, got %d", len(radio.SupportedTAIs[0].SNssais))
			}
		default:
			t.Fatalf("unexpected radio name %q", radio.Name)
		}
	}
}
