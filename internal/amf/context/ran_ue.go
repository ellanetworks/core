package context

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/mohae/deepcopy"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/amf/logger"
)

type RelAction int

const (
	RanUeNgapIdUnspecified int64 = 0xffffffff
)

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
)

type RanUe struct {
	/* UE identity*/
	RanUeNgapId int64 `json:"ranUeNgapId,omitempty"`
	AmfUeNgapId int64 `json:"amfUeNgapId,omitempty"`

	/* HandOver Info*/
	HandOverType        ngapType.HandoverType
	SuccessPduSessionId []int32 `json:"successPduSessionId,omitempty"`
	SourceUe            *RanUe  `json:"-"`
	TargetUe            *RanUe  `json:"-"`

	/* UserLocation*/
	Tai      models.Tai
	Location models.UserLocation
	/* context about udm */
	SupportVoPSn3gpp  bool       `json:"-"`
	SupportVoPS       bool       `json:"-"`
	SupportedFeatures string     `json:"-"`
	LastActTime       *time.Time `json:"-"`

	/* Related Context*/
	AmfUe *AmfUe `json:"-"`
	Ran   *AmfRan

	/* Routing ID */
	RoutingID string
	/* Trace Recording Session Reference */
	Trsr string
	/* Ue Context Release Action */
	ReleaseAction RelAction
	/* context used for AMF Re-allocation procedure */
	OldAmfName            string
	InitialUEMessage      []byte
	RRCEstablishmentCause string // Received from initial ue message; pattern: ^[0-9a-fA-F]+$
	UeContextRequest      bool

	/* send initial context setup request or not*/
	SentInitialContextSetupRequest bool

	/*Received Initial context setup response or not */
	RecvdInitialContextSetupResponse bool

	/* logger */
	Log *logrus.Entry `json:"-"`
}

func (ranUe *RanUe) Remove() error {
	fmt.Printf("RanUe has been deleted")
	if ranUe == nil {
		return fmt.Errorf("RanUe not found in RemoveRanUe")
	}
	ran := ranUe.Ran
	if ran == nil {
		return fmt.Errorf("RanUe not found in Ran")
	}
	if ranUe.AmfUe != nil {
		ranUe.AmfUe.DetachRanUe(ran.AnType)
		ranUe.DetachAmfUe()
	}

	for index, ranUe1 := range ran.RanUeList {
		if ranUe1 == ranUe {
			ran.RanUeList = append(ran.RanUeList[:index], ran.RanUeList[index+1:]...)
			break
		}
	}
	self := AMF_Self()
	self.RanUePool.Delete(ranUe.AmfUeNgapId)
	// amfUeNGAPIDGenerator.FreeID(ranUe.AmfUeNgapId)
	if err := self.Drsm.ReleaseInt32ID(int32(ranUe.AmfUeNgapId)); err != nil {
		logger.ContextLog.Errorf("Error releasing UE: %v", err)
	}
	return nil
}

func (ranUe *RanUe) DetachAmfUe() {
	ranUe.AmfUe = nil
}

func (ranUe *RanUe) SwitchToRan(newRan *AmfRan, ranUeNgapId int64) error {
	if ranUe == nil {
		return fmt.Errorf("ranUe is nil")
	}

	if newRan == nil {
		return fmt.Errorf("newRan is nil")
	}

	oldRan := ranUe.Ran

	// remove ranUe from oldRan
	for index, ranUe1 := range oldRan.RanUeList {
		if ranUe1 == ranUe {
			oldRan.RanUeList = append(oldRan.RanUeList[:index], oldRan.RanUeList[index+1:]...)
			break
		}
	}

	// add ranUe to newRan
	newRan.RanUeList = append(newRan.RanUeList, ranUe)

	// switch to newRan
	ranUe.Ran = newRan
	ranUe.RanUeNgapId = ranUeNgapId

	logger.ContextLog.Infof("RanUe[RanUeNgapID: %d] Switch to new Ran[Name: %s]", ranUe.RanUeNgapId, ranUe.Ran.Name)
	return nil
}

func (ranUe *RanUe) UpdateLocation(userLocationInformation *ngapType.UserLocationInformation) {
	if userLocationInformation == nil {
		return
	}

	amfSelf := AMF_Self()
	curTime := time.Now().UTC()
	switch userLocationInformation.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		locationInfoEUTRA := userLocationInformation.UserLocationInformationEUTRA
		if ranUe.Location.EutraLocation == nil {
			ranUe.Location.EutraLocation = new(models.EutraLocation)
		}

		tAI := locationInfoEUTRA.TAI
		plmnID := ngapConvert.PlmnIdToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.EutraLocation.Tai == nil {
			ranUe.Location.EutraLocation.Tai = new(models.Tai)
		}
		ranUe.Location.EutraLocation.Tai.PlmnId = &plmnID
		ranUe.Location.EutraLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.EutraLocation.Tai

		eUTRACGI := locationInfoEUTRA.EUTRACGI
		ePlmnID := ngapConvert.PlmnIdToModels(eUTRACGI.PLMNIdentity)
		eutraCellID := ngapConvert.BitStringToHex(&eUTRACGI.EUTRACellIdentity.Value)

		if ranUe.Location.EutraLocation.Ecgi == nil {
			ranUe.Location.EutraLocation.Ecgi = new(models.Ecgi)
		}
		ranUe.Location.EutraLocation.Ecgi.PlmnId = &ePlmnID
		ranUe.Location.EutraLocation.Ecgi.EutraCellId = eutraCellID
		ranUe.Location.EutraLocation.UeLocationTimestamp = &curTime
		if locationInfoEUTRA.TimeStamp != nil {
			ranUe.Location.EutraLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(
				locationInfoEUTRA.TimeStamp.Value)
		}
		if ranUe.AmfUe != nil {
			if ranUe.AmfUe.Tai != ranUe.Tai {
				ranUe.AmfUe.LocationChanged = true
			}
			ranUe.AmfUe.Location = deepcopy.Copy(ranUe.Location).(models.UserLocation)
			ranUe.AmfUe.Tai = deepcopy.Copy(*ranUe.AmfUe.Location.EutraLocation.Tai).(models.Tai)
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		locationInfoNR := userLocationInformation.UserLocationInformationNR
		if ranUe.Location.NrLocation == nil {
			ranUe.Location.NrLocation = new(models.NrLocation)
		}

		tAI := locationInfoNR.TAI
		plmnID := ngapConvert.PlmnIdToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.NrLocation.Tai == nil {
			ranUe.Location.NrLocation.Tai = new(models.Tai)
		}
		ranUe.Location.NrLocation.Tai.PlmnId = &plmnID
		ranUe.Location.NrLocation.Tai.Tac = tac
		ranUe.Tai = deepcopy.Copy(*ranUe.Location.NrLocation.Tai).(models.Tai)

		nRCGI := locationInfoNR.NRCGI
		nRPlmnID := ngapConvert.PlmnIdToModels(nRCGI.PLMNIdentity)
		nRCellID := ngapConvert.BitStringToHex(&nRCGI.NRCellIdentity.Value)

		if ranUe.Location.NrLocation.Ncgi == nil {
			ranUe.Location.NrLocation.Ncgi = new(models.Ncgi)
		}
		ranUe.Location.NrLocation.Ncgi.PlmnId = &nRPlmnID
		ranUe.Location.NrLocation.Ncgi.NrCellId = nRCellID
		ranUe.Location.NrLocation.UeLocationTimestamp = &curTime
		if locationInfoNR.TimeStamp != nil {
			ranUe.Location.NrLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(locationInfoNR.TimeStamp.Value)
		}
		if ranUe.AmfUe != nil {
			if ranUe.AmfUe.Tai != ranUe.Tai {
				ranUe.AmfUe.LocationChanged = true
			}
			ranUe.AmfUe.Location = deepcopy.Copy(ranUe.Location).(models.UserLocation)
			ranUe.AmfUe.Tai = deepcopy.Copy(*ranUe.AmfUe.Location.NrLocation.Tai).(models.Tai)
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		locationInfoN3IWF := userLocationInformation.UserLocationInformationN3IWF
		if ranUe.Location.N3gaLocation == nil {
			ranUe.Location.N3gaLocation = new(models.N3gaLocation)
		}

		ip := locationInfoN3IWF.IPAddress
		port := locationInfoN3IWF.PortNumber

		ipv4Addr, ipv6Addr := ngapConvert.IPAddressToString(ip)

		ranUe.Location.N3gaLocation.UeIpv4Addr = ipv4Addr
		ranUe.Location.N3gaLocation.UeIpv6Addr = ipv6Addr
		ranUe.Location.N3gaLocation.PortNumber = ngapConvert.PortNumberToInt(port)
		// N3GPP TAI is operator-specific
		// TODO: define N3GPP TAI
		tmp, err := strconv.ParseUint(amfSelf.SupportTaiLists[0].Tac, 10, 32)
		if err != nil {
			logger.ContextLog.Errorf("Error parsing TAC: %v", err)
		}
		tac := fmt.Sprintf("%06x", tmp)
		ranUe.Location.N3gaLocation.N3gppTai = &models.Tai{
			PlmnId: amfSelf.SupportTaiLists[0].PlmnId,
			Tac:    tac,
		}
		ranUe.Tai = deepcopy.Copy(*ranUe.Location.N3gaLocation.N3gppTai).(models.Tai)

		if ranUe.AmfUe != nil {
			ranUe.AmfUe.Location = deepcopy.Copy(ranUe.Location).(models.UserLocation)
			ranUe.AmfUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai
		}
	case ngapType.UserLocationInformationPresentNothing:
	}
}
