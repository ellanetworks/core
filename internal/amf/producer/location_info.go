package producer

import (
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/amf/context"
	"github.com/yeastengine/canard/internal/amf/logger"
)

func LocationInfoHandler(s1, s2 string, msg interface{}) (interface{}, string, interface{}, interface{}) {
	switch msg := msg.(type) {
	case models.RequestLocInfo:
		r1, r2 := ProvideLocationInfoProcedure(msg, s1)
		return r1, "", r2, nil
	}

	return nil, "", nil, nil
}

func HandleProvideLocationInfoRequest(request *httpwrapper.Request) *httpwrapper.Response {
	var ue *context.AmfUe
	var ok bool
	logger.ProducerLog.Info("Handle Provide Location Info Request")

	requestLocInfo := request.Body.(models.RequestLocInfo)
	ueContextID := request.Params["ueContextId"]

	amfSelf := context.AMF_Self()
	if ue, ok = amfSelf.AmfUeFindByUeContextID(ueContextID); !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}

	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         requestLocInfo,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	var provideLocInfo *models.ProvideLocInfo
	ue.EventChannel.UpdateSbiHandler(LocationInfoHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	if msg.RespData != nil {
		provideLocInfo = msg.RespData.(*models.ProvideLocInfo)
	}
	// provideLocInfo, problemDetails := ProvideLocationInfoProcedure(requestLocInfo, ueContextID)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusOK, nil, provideLocInfo)
	}
}

func ProvideLocationInfoProcedure(requestLocInfo models.RequestLocInfo, ueContextID string) (
	*models.ProvideLocInfo, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, problemDetails
	}

	anType := ue.GetAnType()
	if anType == "" {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, problemDetails
	}

	provideLocInfo := new(models.ProvideLocInfo)

	ranUe := ue.RanUe[anType]
	if requestLocInfo.Req5gsLoc || requestLocInfo.ReqCurrentLoc {
		provideLocInfo.CurrentLoc = true
		provideLocInfo.Location = &ue.Location
	}

	if requestLocInfo.ReqRatType {
		provideLocInfo.RatType = ue.RatType
	}

	if requestLocInfo.ReqTimeZone {
		provideLocInfo.Timezone = ue.TimeZone
	}

	if requestLocInfo.SupportedFeatures != "" {
		provideLocInfo.SupportedFeatures = ranUe.SupportedFeatures
	}
	return provideLocInfo, nil
}
