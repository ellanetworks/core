// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"strings"

	"github.com/omec-project/openapi/models"
)

var policyTriggerArray = []models.PolicyControlRequestTrigger{
	models.PolicyControlRequestTrigger_PLMN_CH,
	models.PolicyControlRequestTrigger_RES_MO_RE,
	models.PolicyControlRequestTrigger_AC_TY_CH,
	models.PolicyControlRequestTrigger_UE_IP_CH,
	models.PolicyControlRequestTrigger_UE_MAC_CH,
	models.PolicyControlRequestTrigger_AN_CH_COR,
	models.PolicyControlRequestTrigger_US_RE,
	models.PolicyControlRequestTrigger_APP_STA,
	models.PolicyControlRequestTrigger_APP_STO,
	models.PolicyControlRequestTrigger_AN_INFO,
	models.PolicyControlRequestTrigger_CM_SES_FAIL,
	models.PolicyControlRequestTrigger_PS_DA_OFF,
	models.PolicyControlRequestTrigger_DEF_QOS_CH,
	models.PolicyControlRequestTrigger_SE_AMBR_CH,
	models.PolicyControlRequestTrigger_QOS_NOTIF,
	models.PolicyControlRequestTrigger_NO_CREDIT,
	models.PolicyControlRequestTrigger_PRA_CH,
	models.PolicyControlRequestTrigger_SAREA_CH,
	models.PolicyControlRequestTrigger_SCNN_CH,
	models.PolicyControlRequestTrigger_RE_TIMEOUT,
	models.PolicyControlRequestTrigger_RES_RELEASE,
	models.PolicyControlRequestTrigger_SUCC_RES_ALLO,
	models.PolicyControlRequestTrigger_RAT_TY_CH,
	models.PolicyControlRequestTrigger_REF_QOS_IND_CH,
	models.PolicyControlRequestTrigger_NUM_OF_PACKET_FILTER,
	models.PolicyControlRequestTrigger_UE_STATUS_RESUME,
	models.PolicyControlRequestTrigger_UE_TZ_CH,
}

// Convert Snssai form models to hexString(sst(2)+sd(6))
func SnssaiModelsToHex(snssai models.Snssai) string {
	// Format sst as a two-digit hex number.
	sst := fmt.Sprintf("%02x", snssai.Sst)
	combined := sst + snssai.Sd

	// Remove all leading '0' characters.
	result := strings.TrimLeft(combined, "0")

	// In case the string was all zeros, return "0" instead of an empty string.
	if result == "" {
		return "0"
	}
	return result
}

// Use BitMap to generate requested policy control triggers,
// 1 means yes, 0 means no, see subscaulse 5.6.3.6-1 in TS29512
func PolicyControlReqTrigToArray(bitMap uint64) (trigger []models.PolicyControlRequestTrigger) {
	cnt := 0
	size := len(policyTriggerArray)
	for bitMap > 0 && cnt < size {
		if (bitMap & 0x01) > 0 {
			trigger = append(trigger, policyTriggerArray[cnt])
		}
		bitMap >>= 1
		cnt++
	}
	return
}
