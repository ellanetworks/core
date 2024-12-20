package qos

import "github.com/omec-project/openapi/models"

type TrafficControlUpdate struct {
	add, mod, del map[string]*models.TrafficControlData
}

func GetTrafficControlUpdate(tcData, ctxtTcData map[string]*models.TrafficControlData) *TrafficControlUpdate {
	if len(tcData) == 0 {
		return nil
	}

	change := TrafficControlUpdate{
		add: make(map[string]*models.TrafficControlData),
		mod: make(map[string]*models.TrafficControlData),
		del: make(map[string]*models.TrafficControlData),
	}

	// Compare against Ctxt rules to get added or modified rules
	for name, pcfTc := range tcData {
		// if pcfRule is nil then it need to be deleted
		if pcfTc == nil {
			change.del[name] = pcfTc // nil
			continue
		}

		// match against SM ctxt Rules for add/mod
		if ctxtTc := ctxtTcData[name]; ctxtTc == nil {
			change.add[name] = pcfTc
		} else if GetTCDataChanges(pcfTc, ctxtTc) {
			change.mod[name] = pcfTc
		}
	}

	return &change
}

func CommitTrafficControlUpdate(smCtxtPolData *SmCtxtPolicyData, update *TrafficControlUpdate) {
	// Iterate through Add/Mod/Del TC

	// Add new tc
	if len(update.add) > 0 {
		for name, tc := range update.add {
			smCtxtPolData.SmCtxtTCData.TrafficControlData[name] = tc
		}
	}

	// Del Rules
	if len(update.del) > 0 {
		for name := range update.del {
			delete(smCtxtPolData.SmCtxtTCData.TrafficControlData, name)
		}
	}
}

func GetTCDataChanges(pcfTc, ctxtTc *models.TrafficControlData) bool {
	return false
}

func GetTcDataFromPolicyDecision(smPolicyDecision *models.SmPolicyDecision, refTcData string) *models.TrafficControlData {
	return smPolicyDecision.TraffContDecs[refTcData]
}
