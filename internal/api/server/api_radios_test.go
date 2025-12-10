package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/models"
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
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
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
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/ran/radios?page=%d&per_page=%d", url, page, perPage), nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var response ListRadiosResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &response, nil
}

func TestListRadios(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	amf := amfContext.AMFSelf()
	ran1 := amfContext.AmfRan{}
	ran1.Name = "gnb-001"
	ran1.SupportedTAList = []amfContext.SupportedTAI{
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
	}
	ran1.GnbIP = "1.2.3.4"
	ran1.GnbID = "mcc:001:mnc:01:gnb-001"

	ran2 := amfContext.AmfRan{}
	ran2.Name = "gnb-002"
	ran2.SupportedTAList = []amfContext.SupportedTAI{
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
	}
	ran2.GnbIP = "2.3.4.5"
	ran2.GnbID = "mcc:001:mnc:01:gnb-002"

	conn1 := sctp.NewSCTPConn(1, nil)
	conn2 := sctp.NewSCTPConn(2, nil)

	amf.AmfRanPool = map[*sctp.SCTPConn]*amfContext.AmfRan{
		conn1: &ran1,
		conn2: &ran2,
	}

	// Set up the Gin router
	statusCode, response, err := listRadios(ts.URL, client, token, 1, 10)
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
		if radio.Name == "gnb-001" {
			if radio.Address != "1.2.3.4" {
				t.Fatalf("expected radio address %q, got %q", "1.2.3.4", radio.Address)
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
		} else if radio.Name == "gnb-002" {
			if radio.Address != "2.3.4.5" {
				t.Fatalf("expected radio address %q, got %q", "2.3.4.5", radio.Address)
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
		} else {
			t.Fatalf("unexpected radio name %q", radio.Name)
		}
	}
}
