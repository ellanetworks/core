// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var smfContext SMFContext

var tracer = otel.Tracer("ella-core/smf")

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionTypeIPv4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

type SMFContext struct {
	DBInstance     *db.Database
	UPF            *UPF
	CPNodeID       net.IP
	LocalSEIDCount uint64
}

// SnssaiSmfDnnInfo records the SMF per S-NSSAI DNN information
type SnssaiSmfDnnInfo struct {
	DNS net.IP
	MTU uint16
}

// SnssaiSmfInfo records the SMF S-NSSAI related information
type SnssaiSmfInfo struct {
	DnnInfos *SnssaiSmfDnnInfo
	Snssai   models.Snssai
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func RetrieveDnnInformation(ctx context.Context, ueSnssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	supportedSnssai, err := GetSnssaiInfo(ctx, dnn)
	if err != nil {
		return nil, fmt.Errorf("failed to get snssai information: %v", err)
	}

	if supportedSnssai.Snssai.Sst != ueSnssai.Sst {
		return nil, fmt.Errorf("ue requested sst %d, but sst %d is supported", ueSnssai.Sst, supportedSnssai.Snssai.Sst)
	}

	if supportedSnssai.Snssai.Sd != ueSnssai.Sd {
		return nil, fmt.Errorf("ue requested sd %s, but sd %s is supported", ueSnssai.Sd, supportedSnssai.Snssai.Sd)
	}

	return supportedSnssai.DnnInfos, nil
}

func AllocateLocalSEID() (uint64, error) {
	atomic.AddUint64(&smfContext.LocalSEIDCount, 1)
	return smfContext.LocalSEIDCount, nil
}

func SMFSelf() *SMFContext {
	return &smfContext
}

func GetSnssaiInfo(ctx context.Context, dnn string) (*SnssaiSmfInfo, error) {
	ctx, span := tracer.Start(ctx, "SMF GetSnssaiInfo")
	defer span.End()
	span.SetAttributes(
		attribute.String("dnn", dnn),
	)

	self := SMFSelf()

	operator, err := self.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}

	dataNetwork, err := self.DBInstance.GetDataNetwork(ctx, dnn)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies from db: %v", err)
	}

	if dataNetwork == nil {
		return nil, fmt.Errorf("data network %s not found", dnn)
	}

	snssaiInfo := &SnssaiSmfInfo{
		Snssai: models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		DnnInfos: &SnssaiSmfDnnInfo{
			DNS: net.ParseIP(dataNetwork.DNS).To4(),
			MTU: uint16(dataNetwork.MTU),
		},
	}

	return snssaiInfo, nil
}

type SubscriberConfig struct {
	DnnConfig *models.DnnConfiguration
	SmPolicy  *models.SmPolicyDecision
}

func GetSubscriberConfig(ctx context.Context, ueID string) (*SubscriberConfig, error) {
	ctx, span := tracer.Start(ctx, "SMF GetSubscriberConfig")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", ueID),
	)

	self := SMFSelf()

	subscriber, err := self.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := self.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	dnnConfig := &models.DnnConfiguration{
		PduSessionTypes: &models.PduSessionTypes{
			DefaultSessionType:  models.PduSessionTypeIPv4,
			AllowedSessionTypes: make([]models.PduSessionType, 0),
		},
		SscModes: &models.SscModes{
			DefaultSscMode:  models.SscMode1,
			AllowedSscModes: make([]models.SscMode, 0),
		},
		SessionAmbr: &models.Ambr{
			Downlink: policy.BitrateDownlink,
			Uplink:   policy.BitrateUplink,
		},
		Var5gQosProfile: &models.SubscribedDefaultQos{
			Var5qi: policy.Var5qi,
			Arp:    &models.Arp{PriorityLevel: policy.Arp},
		},
	}

	dnnConfig.PduSessionTypes.AllowedSessionTypes = append(dnnConfig.PduSessionTypes.AllowedSessionTypes, AllowedSessionTypes...)
	for _, sscMode := range AllowedSscModes {
		dnnConfig.SscModes.AllowedSscModes = append(dnnConfig.SscModes.AllowedSscModes, models.SscMode(sscMode))
	}

	subscriberPolicy := &models.SmPolicyDecision{
		SessRule: &models.SessionRule{
			AuthDefQos: &models.AuthorizedDefaultQos{
				Var5qi: policy.Var5qi,
				Arp:    &models.Arp{PriorityLevel: policy.Arp},
			},
			AuthSessAmbr: &models.Ambr{
				Uplink:   policy.BitrateUplink,
				Downlink: policy.BitrateDownlink,
			},
		},
		QosDecs: &models.QosData{
			Var5qi:               policy.Var5qi,
			Arp:                  &models.Arp{PriorityLevel: policy.Arp},
			DefQosFlowIndication: true,
		},
	}

	subConfig := &SubscriberConfig{
		DnnConfig: dnnConfig,
		SmPolicy:  subscriberPolicy,
	}

	return subConfig, nil
}
