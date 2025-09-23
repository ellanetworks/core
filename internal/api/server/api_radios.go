package server

import (
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
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

type ListRadiosResponse struct {
	Items      []Radio `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

func convertRadioTaiToReturnTai(tais []context.SupportedTAI) []SupportedTAI {
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

func ListRadios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		total, ranList := context.ListAmfRan(page, perPage)

		items := make([]Radio, 0, len(ranList))

		for _, radio := range ranList {
			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
			newRadio := Radio{
				Name:          radio.Name,
				ID:            radio.GnbID,
				Address:       radio.GnbIP,
				SupportedTAIs: supportedTais,
			}

			items = append(items, newRadio)
		}

		resp := ListRadiosResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, resp, http.StatusOK, logger.APILog)
	}
}

func GetRadio() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		radioName := pathParam(r.URL.Path, "/api/v1/radios/")
		if radioName == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", fmt.Errorf("name parameter is required"), logger.APILog)
			return
		}

		_, ranList := context.ListAmfRan(1, 1000)

		for _, radio := range ranList {
			if radio.Name == radioName {
				supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
				result := Radio{
					Name:          radio.Name,
					ID:            radio.GnbID,
					Address:       radio.GnbIP,
					SupportedTAIs: supportedTais,
				}
				writeResponse(w, result, http.StatusOK, logger.APILog)
				return
			}
		}

		writeError(w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
	}
}
