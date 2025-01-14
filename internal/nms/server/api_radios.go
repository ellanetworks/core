package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type PlmnId struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnId PlmnId `json:"plmnID"`
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
	Id            string         `json:"id"`
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
				PlmnId: PlmnId{
					Mcc: tai.Tai.PlmnId.Mcc,
					Mnc: tai.Tai.PlmnId.Mnc,
				},
				Tac: tai.Tai.Tac,
			},
			SNssais: snssais,
		}
		returnedTais = append(returnedTais, newTai)
	}
	return returnedTais
}

func ListRadios() gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}

		ranList := context.ListAmfRan()
		radios := make([]GetRadioParams, 0)
		for _, radio := range ranList {
			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
			newRadio := GetRadioParams{
				Name:          radio.Name,
				Id:            radio.GnbId,
				Address:       radio.GnbIp,
				SupportedTAIs: supportedTais,
			}
			radios = append(radios, newRadio)
		}

		err := writeResponse(c.Writer, radios, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			ListRadiosAction,
			email,
			"User listed radios",
		)
	}
}

func GetRadio() gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		radioName, exists := c.Params.Get("name")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		ranList := context.ListAmfRan()
		var returnRadio GetRadioParams
		for _, radio := range ranList {
			if radio.Name == radioName {
				supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
				returnRadio = GetRadioParams{
					Name:          radio.Name,
					Id:            radio.GnbId,
					Address:       radio.GnbIp,
					SupportedTAIs: supportedTais,
				}
				break
			}
		}

		err := writeResponse(c.Writer, returnRadio, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetRadioAction,
			email,
			"User retrieved radio: "+radioName,
		)
	}
}
