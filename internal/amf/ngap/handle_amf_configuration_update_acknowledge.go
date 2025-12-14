package ngap

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func HandleAMFconfigurationUpdateAcknowledge(ran *context.AmfRan, message *ngapType.NGAPPDU) {
	var aMFTNLAssociationSetupList *ngapType.AMFTNLAssociationSetupList
	var criticalityDiagnostics *ngapType.CriticalityDiagnostics
	var aMFTNLAssociationFailedToSetupList *ngapType.TNLAssociationList
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}
	successfulOutcome := message.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}
	aMFConfigurationUpdateAcknowledge := successfulOutcome.Value.AMFConfigurationUpdateAcknowledge
	if aMFConfigurationUpdateAcknowledge == nil {
		ran.Log.Error("AMFConfigurationUpdateAcknowledge is nil")
		return
	}

	for i := 0; i < len(aMFConfigurationUpdateAcknowledge.ProtocolIEs.List); i++ {
		ie := aMFConfigurationUpdateAcknowledge.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
			aMFTNLAssociationSetupList = ie.Value.AMFTNLAssociationSetupList
			if aMFTNLAssociationSetupList == nil {
				ran.Log.Error("AMFTNLAssociationSetupList is nil")
				return
			}
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics

		case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
			aMFTNLAssociationFailedToSetupList = ie.Value.AMFTNLAssociationFailedToSetupList
			if aMFTNLAssociationFailedToSetupList == nil {
				ran.Log.Error("AMFTNLAssociationFailedToSetupList is nil")
				return
			}
		}
	}

	if criticalityDiagnostics != nil {
		printCriticalityDiagnostics(ran, criticalityDiagnostics)
	}
}
