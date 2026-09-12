// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
)

const (
	RadioStatusOnline  = "online"
	RadioStatusOffline = "offline"
)

func radioStatus(connected bool) string {
	if connected {
		return RadioStatusOnline
	}

	return RadioStatusOffline
}

func matchesRadioStatus(filter string, connected bool) bool {
	return filter == "" || filter == radioStatus(connected)
}

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
	Name           string  `json:"name"`
	Ref            string  `json:"ref"`
	ID             string  `json:"id"`
	PlmnID         *PlmnID `json:"plmn,omitempty"`
	BitLength      *int32  `json:"bit_length,omitempty"`
	Address        string  `json:"address"`
	RanNodeType    string  `json:"type"`
	Status         string  `json:"status"`
	ConnectedAt    string  `json:"connected_at"`
	LastSeenAt     string  `json:"last_seen_at"`
	DisconnectedAt string  `json:"disconnected_at"`
	// Deprecated: Use the GET /api/v1/ran/radios/{name} detail endpoint instead.
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

const ForgetRadioAction = "forget_radio"

type ListRadiosResponse struct {
	Items      []Radio `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

type RadioDetail struct {
	Name           string         `json:"name"`
	Ref            string         `json:"ref"`
	ID             string         `json:"id"`
	PlmnID         *PlmnID        `json:"plmn,omitempty"`
	BitLength      *int32         `json:"bit_length,omitempty"`
	Address        string         `json:"address"`
	Status         string         `json:"status"`
	ConnectedAt    string         `json:"connected_at"`
	LastSeenAt     string         `json:"last_seen_at"`
	DisconnectedAt string         `json:"disconnected_at"`
	RanNodeType    string         `json:"type"`
	SupportedTAIs  []SupportedTAI `json:"supported_tais"`
}

func convertPlmnID(plmn *models.PlmnID) *PlmnID {
	if plmn == nil {
		return nil
	}

	return &PlmnID{Mcc: plmn.Mcc, Mnc: plmn.Mnc}
}

func formatRadioTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
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
func convertENBTaiToReturnTai(tais []mme.SupportedTAI) []SupportedTAI {
	returnedTais := make([]SupportedTAI, 0, len(tais))
	for _, tai := range tais {
		returnedTais = append(returnedTais, SupportedTAI{
			Tai: Tai{
				PlmnID: PlmnID{
					Mcc: tai.Tai.PlmnID.Mcc,
					Mnc: tai.Tai.PlmnID.Mnc,
				},
				Tac: tai.Tai.Tac,
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

		status := q.Get("status")
		if status != "" && status != RadioStatusOnline && status != RadioStatusOffline {
			writeError(r.Context(), w, http.StatusBadRequest, "status must be online or offline", nil, logger.APILog)

			return
		}

		// 4G eNBs and 5G gNBs share one radio namespace, distinguished by node type.
		// Combine both RATs' full lists and paginate the whole so pagination is
		// consistent across them (each RAT exposes an all-radios ListRadios()).
		items := make([]Radio, 0)

		for _, radio := range amfInstance.ListRadios() {
			if !matchesRadioStatus(status, radio.Connected) {
				continue
			}

			items = append(items, Radio{
				Name:           radio.Name,
				Ref:            radio.Ref,
				ID:             radio.ID,
				PlmnID:         convertPlmnID(radio.PlmnID),
				BitLength:      radio.BitLength,
				Address:        radio.Address,
				RanNodeType:    radio.RanNodeType,
				Status:         radioStatus(radio.Connected),
				ConnectedAt:    formatRadioTime(radio.ConnectedAt),
				LastSeenAt:     formatRadioTime(radio.LastSeenAt),
				DisconnectedAt: formatRadioTime(radio.DisconnectedAt),
				SupportedTAIs:  convertRadioTaiToReturnTai(radio.SupportedTAIs),
			})
		}

		if mmeInstance != nil {
			for _, enb := range mmeInstance.ListRadios() {
				if !matchesRadioStatus(status, enb.Connected) {
					continue
				}

				items = append(items, Radio{
					Name:           enb.Name,
					Ref:            enb.Ref,
					ID:             enb.ID,
					PlmnID:         convertPlmnID(enb.PlmnID),
					Address:        enb.Address,
					RanNodeType:    enb.RanNodeType,
					Status:         radioStatus(enb.Connected),
					ConnectedAt:    formatRadioTime(enb.ConnectedAt),
					LastSeenAt:     formatRadioTime(enb.LastSeenAt),
					DisconnectedAt: formatRadioTime(enb.DisconnectedAt),
					SupportedTAIs:  convertENBTaiToReturnTai(enb.SupportedTAIs),
				})
			}
		}

		slices.SortFunc(items, func(a, b Radio) int {
			return cmp.Or(
				cmp.Compare(a.Name, b.Name),
				cmp.Compare(a.RanNodeType, b.RanNodeType),
				cmp.Compare(a.ID, b.ID),
			)
		})

		total := len(items)

		start := (page - 1) * perPage
		end := start + perPage

		if start > total {
			start = total
		}

		if end > total {
			end = total
		}

		resp := ListRadiosResponse{
			Items:      items[start:end],
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	}
}

func radioPathIdentity(r *http.Request) (models.GlobalRanNodeID, error) {
	ref := r.PathValue("ref")
	if ref == "" {
		return models.GlobalRanNodeID{}, fmt.Errorf("ref parameter is required")
	}

	return models.ParseRanNodeRef(ref)
}

func GetRadio(amfInstance *amf.AMF, mmeInstance *mme.MME) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ranID, err := radioPathIdentity(r)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing radio identity", err, logger.APILog)
			return
		}

		if ranID.ENbID != "" {
			if mmeInstance == nil {
				writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
				return
			}

			enb, ok := mmeInstance.FindRadioInfoByRanID(ranID)
			if !ok {
				writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
				return
			}

			result := RadioDetail{
				Name:           enb.Name,
				Ref:            enb.Ref,
				ID:             enb.ID,
				PlmnID:         convertPlmnID(enb.PlmnID),
				Address:        enb.Address,
				Status:         radioStatus(enb.Connected),
				ConnectedAt:    formatRadioTime(enb.ConnectedAt),
				LastSeenAt:     formatRadioTime(enb.LastSeenAt),
				DisconnectedAt: formatRadioTime(enb.DisconnectedAt),
				RanNodeType:    enb.RanNodeType,
				SupportedTAIs:  convertENBTaiToReturnTai(enb.SupportedTAIs),
			}

			writeResponse(r.Context(), w, result, http.StatusOK, logger.APILog)

			return
		}

		radio, ok := amfInstance.FindRadioInfoByRanID(ranID)
		if !ok {
			writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio not found"), logger.APILog)
			return
		}

		result := RadioDetail{
			Name:           radio.Name,
			Ref:            radio.Ref,
			ID:             radio.ID,
			PlmnID:         convertPlmnID(radio.PlmnID),
			BitLength:      radio.BitLength,
			Address:        radio.Address,
			Status:         radioStatus(radio.Connected),
			ConnectedAt:    formatRadioTime(radio.ConnectedAt),
			LastSeenAt:     formatRadioTime(radio.LastSeenAt),
			DisconnectedAt: formatRadioTime(radio.DisconnectedAt),
			RanNodeType:    radio.RanNodeType,
			SupportedTAIs:  convertRadioTaiToReturnTai(radio.SupportedTAIs),
		}

		writeResponse(r.Context(), w, result, http.StatusOK, logger.APILog)
	}
}

func ForgetRadio(amfInstance *amf.AMF, mmeInstance *mme.MME) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		ranID, err := radioPathIdentity(r)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing radio identity", err, logger.APILog)
			return
		}

		forgetErr := amf.ErrRadioNotFound

		switch {
		case ranID.ENbID == "":
			forgetErr = amfInstance.ForgetRadio(ranID)
		case mmeInstance != nil:
			forgetErr = mmeInstance.ForgetRadio(ranID)
		}

		switch {
		case errors.Is(forgetErr, amf.ErrRadioOnline) || errors.Is(forgetErr, mme.ErrRadioOnline):
			writeError(r.Context(), w, http.StatusConflict, "Radio is online", nil, logger.APILog)

			return
		case forgetErr != nil:
			writeError(r.Context(), w, http.StatusNotFound, "Radio not found", nil, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Radio forgotten successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), ForgetRadioAction, email, getClientIP(r), "User forgot radio: "+ranID.String())
	}
}
