package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/omec-project/openapi/models"
)

const (
	RadioName = "gnb-001"
)

type PlmnId struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnId PlmnId `json:"plmnId"`
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

type GetRadioResponseResult struct {
	Name          string         `json:"name"`
	Id            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

type GetRadioResponse struct {
	Result GetRadioResponseResult `json:"result"`
	Error  string                 `json:"error,omitempty"`
}

type ListRadiosResponse struct {
	Result []GetRadioResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func listRadios(url string, client *http.Client, token string) (int, *ListRadiosResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/radios", nil)
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
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	amf := amfContext.AMF_Self()
	ran1 := amfContext.AmfRan{}
	ran1.Name = "gnb-001"
	ran1.SupportedTAList = []amfContext.SupportedTAI{
		{
			Tai: models.Tai{
				PlmnId: &models.PlmnId{
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
	ran1.GnbIp = "1.2.3.4"
	ran1.GnbId = "mcc:001:mnc:01:gnb-001"
	amf.AmfRanPool.Store("id1", &ran1)
	ran2 := amfContext.AmfRan{}
	ran2.Name = "gnb-002"
	ran2.SupportedTAList = []amfContext.SupportedTAI{
		{
			Tai: models.Tai{
				PlmnId: &models.PlmnId{
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
	ran2.GnbIp = "2.3.4.5"
	ran2.GnbId = "mcc:001:mnc:01:gnb-002"
	amf.AmfRanPool.Store("id2", &ran2)

	// Set up the Gin router
	statusCode, response, err := listRadios(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't list profile: %s", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	if len(response.Result) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(response.Result))
	}

	fmt.Println("Result: ", response.Result)

	if response.Result[0].Name != "gnb-001" {
		t.Fatalf("expected radio name %q, got %q", "gnb-001", response.Result[0].Name)
	}

	if response.Result[0].Address != "1.2.3.4" {
		t.Fatalf("expected radio address %q, got %q", "1.2.3.4", response.Result[0].Address)
	}

	if response.Result[0].Id != "mcc:001:mnc:01:gnb-001" {
		t.Fatalf("expected radio ID %q, got %q", "mcc:001:mnc:01:gnb-001", response.Result[0].Id)
	}

	if len(response.Result[0].SupportedTAIs) != 1 {
		t.Fatalf("expected 1 supported TAI, got %d", len(response.Result[0].SupportedTAIs))
	}

	if response.Result[0].SupportedTAIs[0].Tai.PlmnId.Mcc != "123" {
		t.Fatalf("expected mcc %q, got %q", "123", response.Result[0].SupportedTAIs[0].Tai.PlmnId.Mcc)
	}

	if response.Result[0].SupportedTAIs[0].Tai.PlmnId.Mnc != "12" {
		t.Fatalf("expected mnc %q, got %q", "12", response.Result[0].SupportedTAIs[0].Tai.PlmnId.Mnc)
	}

	if response.Result[0].SupportedTAIs[0].Tai.Tac != "0002" {
		t.Fatalf("expected tac %q, got %q", "0002", response.Result[0].SupportedTAIs[0].Tai.Tac)
	}

	if len(response.Result[0].SupportedTAIs[0].SNssais) != 1 {
		t.Fatalf("expected 1 supported SNssai, got %d", len(response.Result[0].SupportedTAIs[0].SNssais))
	}

	if response.Result[0].SupportedTAIs[0].SNssais[0].Sst != 2 {
		t.Fatalf("expected sst %d, got %d", 2, response.Result[0].SupportedTAIs[0].SNssais[0].Sst)
	}

	if response.Result[0].SupportedTAIs[0].SNssais[0].Sd != "010204" {
		t.Fatalf("expected sd %q, got %q", "010204", response.Result[0].SupportedTAIs[0].SNssais[0].Sd)
	}

	if response.Result[1].Name != "gnb-002" {
		t.Fatalf("expected radio name %q, got %q", "gnb-002", response.Result[1].Name)
	}

	if response.Result[1].Address != "2.3.4.5" {
		t.Fatalf("expected radio address %q, got %q", "2.3.4.5", response.Result[1].Address)
	}

	if response.Result[1].Id != "mcc:001:mnc:01:gnb-002" {
		t.Fatalf("expected radio ID %q, got %q", "mcc:001:mnc:01:gnb-002", response.Result[1].Id)
	}

	if len(response.Result[1].SupportedTAIs) != 1 {
		t.Fatalf("expected 1 supported TAI, got %d", len(response.Result[1].SupportedTAIs))
	}

	if response.Result[1].SupportedTAIs[0].Tai.PlmnId.Mcc != "001" {
		t.Fatalf("expected mcc %q, got %q", "001", response.Result[1].SupportedTAIs[0].Tai.PlmnId.Mcc)
	}

	if response.Result[1].SupportedTAIs[0].Tai.PlmnId.Mnc != "01" {
		t.Fatalf("expected mnc %q, got %q", "01", response.Result[1].SupportedTAIs[0].Tai.PlmnId.Mnc)
	}

	if response.Result[1].SupportedTAIs[0].Tai.Tac != "0001" {
		t.Fatalf("expected tac %q, got %q", "0001", response.Result[1].SupportedTAIs[0].Tai.Tac)
	}

	if len(response.Result[1].SupportedTAIs[0].SNssais) != 1 {
		t.Fatalf("expected 1 supported SNssai, got %d", len(response.Result[1].SupportedTAIs[0].SNssais))
	}
}
