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

type GetRadioParams struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

const (
	ListRadiosAction = "list_radios"
	GetRadioAction   = "get_radio"
)

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
		email := r.Context().Value(contextKeyEmail)
		emailStr, ok := email.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", fmt.Errorf("missing email in context"), logger.APILog)
			return
		}

		ranList := context.ListAmfRan()
		radios := make([]GetRadioParams, 0, len(ranList))
		for _, radio := range ranList {
			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
			newRadio := GetRadioParams{
				Name:          radio.Name,
				ID:            radio.GnbID,
				Address:       radio.GnbIP,
				SupportedTAIs: supportedTais,
			}
			radios = append(radios, newRadio)
		}

		writeResponse(w, radios, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			ListRadiosAction,
			emailStr,
			getClientIP(r),
			"User listed radios",
		)
	}
}

func GetRadio() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)
		emailStr, ok := email.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", fmt.Errorf("missing email in context"), logger.APILog)
			return
		}

		radioName := pathParam(r.URL.Path, "/api/v1/radios/")
		if radioName == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", fmt.Errorf("name parameter is required"), logger.APILog)
			return
		}

		ranList := context.ListAmfRan()
		for _, radio := range ranList {
			if radio.Name == radioName {
				supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
				result := GetRadioParams{
					Name:          radio.Name,
					ID:            radio.GnbID,
					Address:       radio.GnbIP,
					SupportedTAIs: supportedTais,
				}
				writeResponse(w, result, http.StatusOK, logger.APILog)
				logger.LogAuditEvent(
					GetRadioAction,
					emailStr,
					getClientIP(r),
					"User retrieved radio: "+radioName,
				)
				return
			}
		}

		writeError(w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
	}
}
