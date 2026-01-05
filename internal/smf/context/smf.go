// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var smfContext SMF

var tracer = otel.Tracer("ella-core/smf")

type SMF struct {
	Mutex sync.Mutex

	DBInstance         *db.Database
	CPNodeID           net.IP
	LocalSEIDCount     uint64
	allowedSessionType uint8
	smContextPool      map[string]*SMContext // key: canonicalName(identifier, pduSessID)
	pdrIDGenerator     *idgenerator.IDGenerator
	farIDGenerator     *idgenerator.IDGenerator
	qerIDGenerator     *idgenerator.IDGenerator
	urrIDGenerator     *idgenerator.IDGenerator
}

func InitializeSMF(dbInstance *db.Database) {
	smfContext = SMF{
		smContextPool:      make(map[string]*SMContext),
		DBInstance:         dbInstance,
		CPNodeID:           net.ParseIP("0.0.0.0"),
		allowedSessionType: nasMessage.PDUSessionTypeIPv4,
		pdrIDGenerator:     idgenerator.NewGenerator(1, math.MaxUint16),
		farIDGenerator:     idgenerator.NewGenerator(1, math.MaxUint32),
		qerIDGenerator:     idgenerator.NewGenerator(1, math.MaxUint32),
		urrIDGenerator:     idgenerator.NewGenerator(1, math.MaxUint32),
	}
}

type SnssaiSmfDnnInfo struct {
	DNS net.IP
	MTU uint16
}

type SnssaiSmfInfo struct {
	DnnInfos *SnssaiSmfDnnInfo
	Snssai   models.Snssai
}

// RetrieveDnnInformation gets the corresponding dnn info from S-NSSAI and DNN
func (smf *SMF) RetrieveDnnInformation(ctx context.Context, ueSnssai models.Snssai, dnn string) (*SnssaiSmfDnnInfo, error) {
	supportedSnssai, err := smf.GetSnssaiInfo(ctx, dnn)
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

func (smf *SMF) AllocateLocalSEID() uint64 {
	return atomic.AddUint64(&smf.LocalSEIDCount, 1)
}

func SMFSelf() *SMF {
	return &smfContext
}

func (smf *SMF) GetSnssaiInfo(ctx context.Context, dnn string) (*SnssaiSmfInfo, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF GetSnssaiInfo",
		trace.WithAttributes(
			attribute.String("dnn", dnn),
		),
	)
	defer span.End()

	operator, err := smf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator information from db: %v", err)
	}

	dataNetwork, err := smf.DBInstance.GetDataNetwork(ctx, dnn)
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

func (smf *SMF) GetAllowedSessionType() uint8 {
	return smf.allowedSessionType
}

func (smf *SMF) GetSubscriberPolicy(ctx context.Context, ueID string) (*models.SmPolicyDecision, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF GetSubscriberPolicy",
		trace.WithAttributes(
			attribute.String("ue.supi", ueID),
		),
	)
	defer span.End()

	subscriber, err := smf.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := smf.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	subscriberPolicy := &models.SmPolicyDecision{
		SessionRule: &models.SessionRule{
			AuthDefQos: &models.AuthorizedDefaultQos{
				Var5qi: policy.Var5qi,
				Arp:    &models.Arp{PriorityLevel: policy.Arp},
			},
			AuthSessAmbr: &models.Ambr{
				Uplink:   policy.BitrateUplink,
				Downlink: policy.BitrateDownlink,
			},
		},
		QosData: &models.QosData{
			Var5qi: policy.Var5qi,
			Arp:    &models.Arp{PriorityLevel: policy.Arp},
			QFI:    qos.DefaultQFI,
		},
	}

	return subscriberPolicy, nil
}

func (smf *SMF) PDUSessionsByDNN(dnn string) []*SMContext {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	var out []*SMContext

	for _, smContext := range smf.smContextPool {
		if smContext.Dnn == dnn {
			out = append(out, smContext)
		}
	}

	return out
}

func (smf *SMF) GetSMContextBySEID(seid uint64) *SMContext {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	for _, smContext := range smf.smContextPool {
		if smContext.PFCPContext != nil && smContext.PFCPContext.LocalSEID == seid {
			return smContext
		}
	}

	return nil
}

func (smContext *SMContext) AllocateLocalSEIDForDataPath(smf *SMF) {
	if smContext.PFCPContext != nil {
		return
	}

	smContext.PFCPContext = &PFCPSessionContext{
		LocalSEID: smf.AllocateLocalSEID(),
	}
}

func (smf *SMF) NewSMContext(supi string, pduSessID uint8) *SMContext {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	smContext := &SMContext{
		PDUSessionID: pduSessID,
	}

	ref := CanonicalName(supi, pduSessID)
	smf.smContextPool[ref] = smContext

	return smContext
}

func (smf *SMF) GetSMContext(ref string) *SMContext {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	value, ok := smf.smContextPool[ref]
	if !ok {
		return nil
	}

	return value
}

func (smf *SMF) GetPDUSessionCount() int {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	return len(smf.smContextPool)
}

func (smf *SMF) RemoveSMContext(ctx context.Context, ref string) {
	smf.Mutex.Lock()
	defer smf.Mutex.Unlock()

	smContext, ok := smf.smContextPool[ref]
	if !ok {
		return
	}

	err := smf.ReleaseUeIPAddr(ctx, smContext.Supi)
	if err != nil {
		logger.SmfLog.Error("release UE IP-Address failed", zap.Error(err), zap.String("smContextRef", ref))
	}

	delete(smf.smContextPool, ref)

	logger.SmfLog.Info("SM Context removed", zap.String("smContextRef", ref))
}

func (smf *SMF) ReleaseUeIPAddr(ctx context.Context, supi string) error {
	err := smf.DBInstance.ReleaseIP(ctx, supi)
	if err != nil {
		return fmt.Errorf("failed to release IP Address, %v", err)
	}

	logger.SmfLog.Info("Released IP Address", zap.String("supi", supi))

	return nil
}

func (smf *SMF) NewPDR() (*PDR, error) {
	pdrID, err := smf.pdrIDGenerator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate PDR ID: %v", err)
	}

	far, err := smf.NewFAR()
	if err != nil {
		return nil, err
	}

	pdr := &PDR{
		PDRID: uint16(pdrID),
		FAR:   far,
	}

	return pdr, nil
}

func (smf *SMF) NewFAR() (*FAR, error) {
	farID, err := smf.farIDGenerator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate FAR ID: %v", err)
	}

	far := &FAR{
		FARID: uint32(farID),
		ApplyAction: ApplyAction{
			Drop: true,
		},
	}

	return far, nil
}

func (smf *SMF) NewQER(smData *models.SmPolicyDecision) (*QER, error) {
	qerID, err := smf.qerIDGenerator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate QER ID: %v", err)
	}

	qer := &QER{
		QERID: uint32(qerID),
		QFI:   smData.QosData.QFI,
		GateStatus: &GateStatus{
			ULGate: GateOpen,
			DLGate: GateOpen,
		},
		MBR: &MBR{
			ULMBR: BitRateTokbps(smData.SessionRule.AuthSessAmbr.Uplink),
			DLMBR: BitRateTokbps(smData.SessionRule.AuthSessAmbr.Downlink),
		},
	}

	return qer, nil
}

func (smf *SMF) NewURR() (*URR, error) {
	urrID, err := smf.urrIDGenerator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate URR ID: %v", err)
	}

	urr := &URR{
		URRID: uint32(urrID),
		MeasurementMethods: MeasurementMethods{
			Volume: true,
		},
		ReportingTriggers: ReportingTriggers{
			PeriodicReporting: true,
		},
		MeasurementPeriod: 60 * time.Second,
	}

	return urr, nil
}

func (smf *SMF) RemovePDR(pdr *PDR) {
	smf.pdrIDGenerator.FreeID(int64(pdr.PDRID))
}

func (smf *SMF) RemoveFAR(far *FAR) {
	smf.farIDGenerator.FreeID(int64(far.FARID))
}

func (smf *SMF) RemoveQER(qer *QER) {
	smf.qerIDGenerator.FreeID(int64(qer.QERID))
}

func (smf *SMF) RemoveURR(urr *URR) {
	smf.urrIDGenerator.FreeID(int64(urr.URRID))
}
