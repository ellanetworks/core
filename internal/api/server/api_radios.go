// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
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
	Name        string `json:"name"`
	ID          string `json:"id"`
	Address     string `json:"address"`
	RanNodeType string `json:"type"`
	// Deprecated: Use the GET /api/v1/ran/radios/{name} detail endpoint instead.
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

type ListRadiosResponse struct {
	Items      []Radio `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

type RadioDetail struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	ConnectedAt   string         `json:"connected_at"`
	LastSeenAt    string         `json:"last_seen_at"`
	RanNodeType   string         `json:"type"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

func convertRadioTaiToReturnTai(tais []amf.SupportedTAI) []SupportedTAI {
	returnedTais := make([]SupportedTAI, 0)
	for _, tai := range tais {
		snssais := make([]Snssai, 0)

		for _, snssai := range tai.SNssaiList {
			newSnssai := Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			}
			snssais = append(snssais, newSnssai)
		}

		newTai := SupportedTAI{
			Tai: Tai{
				PlmnID: PlmnID{
					Mcc: tai.Tai.PlmnID.Mcc,
					Mnc: tai.Tai.PlmnID.Mnc,
				},
				Tac: tai.Tai.Tac,
			},
			SNssais: snssais,
		}
		returnedTais = append(returnedTais, newTai)
	}

	return returnedTais
}

// convertENBTaiToReturnTai renders a 4G eNB's broadcast TAIs in the radio API
// shape. The 16-bit S1AP TAC is rendered as the low two octets of a 6-hex-digit
// TAC, matching how gNB TAIs and the operator's supported TACs are represented
// (TS 23.003: the LTE TAC is the 5GS TAC's two least-significant octets). eNBs
// carry no S-NSSAIs.
func convertENBTaiToReturnTai(tais []mme.ENBTAI) []SupportedTAI {
	returnedTais := make([]SupportedTAI, 0, len(tais))
	for _, tai := range tais {
		returnedTais = append(returnedTais, SupportedTAI{
			Tai: Tai{
				PlmnID: PlmnID{
					Mcc: tai.PlmnID.Mcc,
					Mnc: tai.PlmnID.Mnc,
				},
				Tac: fmt.Sprintf("%06x", tai.TAC),
			},
			SNssais: []Snssai{},
		})
	}

	return returnedTais
}

func ListRadios(amfInstance *amf.AMF, mmeInstance *mme.MME) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		total, ranList := amfInstance.ListAmfRan(page, perPage)

		items := make([]Radio, 0, len(ranList))

		for _, radio := range ranList {
			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAIs)

			radioAddress := ""

			if radio.Conn != nil {
				if addr := radio.RemoteAddr(); addr != nil {
					radioAddress = addr.String()
				}
			}

			radioID := radio.NodeID()

			newRadio := Radio{
				Name:          radio.Name,
				ID:            radioID,
				Address:       radioAddress,
				RanNodeType:   radio.RanNodeTypeName(),
				SupportedTAIs: supportedTais,
			}

			items = append(items, newRadio)
		}

		// 4G eNBs connected to the MME appear in the same radio list,
		// distinguished by type.
		if mmeInstance != nil {
			for _, enb := range mmeInstance.ListENBs() {
				items = append(items, Radio{
					Name:          enb.Name,
					ID:            enb.ID,
					Address:       enb.Address,
					RanNodeType:   "eNB",
					SupportedTAIs: convertENBTaiToReturnTai(enb.SupportedTAIs),
				})
				total++
			}
		}

		resp := ListRadiosResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	}
}

func GetRadio(amfInstance *amf.AMF, mmeInstance *mme.MME) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		radioName := r.PathValue("name")
		if radioName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", fmt.Errorf("name parameter is required"), logger.APILog)
			return
		}

		_, ranList := amfInstance.ListAmfRan(1, 1000)

		for _, radio := range ranList {
			if radio.Name != radioName {
				continue
			}

			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAIs)

			radioAddress := ""

			if radio.Conn != nil {
				if addr := radio.RemoteAddr(); addr != nil {
					radioAddress = addr.String()
				}
			}

			radioID := radio.NodeID()

			result := RadioDetail{
				Name:          radio.Name,
				ID:            radioID,
				Address:       radioAddress,
				ConnectedAt:   radio.ConnectedAt.UTC().Format(time.RFC3339),
				LastSeenAt:    radio.LastSeenAt().UTC().Format(time.RFC3339),
				RanNodeType:   radio.RanNodeTypeName(),
				SupportedTAIs: supportedTais,
			}

			writeResponse(r.Context(), w, result, http.StatusOK, logger.APILog)

			return
		}

		// 4G eNBs connected to the MME share the radio namespace, distinguished
		// by type.
		if mmeInstance != nil {
			for _, enb := range mmeInstance.ListENBs() {
				if enb.Name != radioName {
					continue
				}

				result := RadioDetail{
					Name:          enb.Name,
					ID:            enb.ID,
					Address:       enb.Address,
					ConnectedAt:   enb.ConnectedAt.UTC().Format(time.RFC3339),
					LastSeenAt:    enb.LastSeenAt.UTC().Format(time.RFC3339),
					RanNodeType:   "eNB",
					SupportedTAIs: convertENBTaiToReturnTai(enb.SupportedTAIs),
				}

				writeResponse(r.Context(), w, result, http.StatusOK, logger.APILog)

				return
			}
		}

		writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
	}
}
