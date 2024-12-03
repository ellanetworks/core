package db

import (
	"go.mongodb.org/mongo-driver/bson"
)

func DeleteSmData(imsi string, mcc string, mnc string) error {
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	err := CommonDBClient.RestfulAPIDeleteOne(SmDataColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateSmProvisionedData(snssai *Snssai, qos *DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, dnn, imsi string) error {
	smData := SessionManagementSubscriptionData{
		SingleNssai: snssai,
		DnnConfigurations: map[string]DnnConfiguration{
			dnn: {
				PduSessionTypes: &PduSessionTypes{
					DefaultSessionType:  PduSessionType_IPV4,
					AllowedSessionTypes: []PduSessionType{PduSessionType_IPV4},
				},
				SscModes: &SscModes{
					DefaultSscMode: SscMode__1,
					AllowedSscModes: []SscMode{
						"SSC_MODE_2",
						"SSC_MODE_3",
					},
				},
				SessionAmbr: &Ambr{
					Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
					Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
				},
				Var5gQosProfile: &SubscribedDefaultQos{
					Var5qi: 9,
					Arp: &Arp{
						PriorityLevel: 8,
					},
					PriorityLevel: 8,
				},
			},
		},
	}
	smDataBsonA := toBsonM(smData)
	smDataBsonA["ueId"] = "imsi-" + imsi
	smDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	_, err := CommonDBClient.RestfulAPIPost(SmDataColl, filter, smDataBsonA)
	if err != nil {
		return err
	}
	return nil
}
