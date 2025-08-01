// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/omec-project/nas/nasMessage"
)

type UPTunnel struct {
	DataPath      *DataPath
	ANInformation struct {
		IPAddress net.IP
		TEID      uint32
	}
}

type RecoveryTimeStamp struct {
	RecoveryTimeStamp time.Time
}

type UPF struct {
	N3Interface UPFInterfaceInfo

	pdrPool sync.Map
	farPool sync.Map
	barPool sync.Map
	qerPool sync.Map

	pdrIDGenerator *idgenerator.IDGenerator
	farIDGenerator *idgenerator.IDGenerator
	barIDGenerator *idgenerator.IDGenerator
	qerIDGenerator *idgenerator.IDGenerator

	NodeID NodeID

	// lock
	UpfLock sync.RWMutex
}

// UPFInterfaceInfo store the UPF interface information
type UPFInterfaceInfo struct {
	IPv4EndPointAddresses []net.IP
}

// IP returns the IP of the user plane IP information of the pduSessType
func (i *UPFInterfaceInfo) IP(pduSessType uint8) (net.IP, error) {
	if (pduSessType == nasMessage.PDUSessionTypeIPv4 || pduSessType == nasMessage.PDUSessionTypeIPv4IPv6) && len(i.IPv4EndPointAddresses) != 0 {
		return i.IPv4EndPointAddresses[0].To4(), nil
	}

	return nil, errors.New("not matched ip address")
}

func NewUPF(nodeID *NodeID) (upf *UPF) {
	upf = new(UPF)
	upf.NodeID = *nodeID
	upf.pdrIDGenerator = idgenerator.NewGenerator(1, math.MaxUint16)
	upf.farIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)
	upf.barIDGenerator = idgenerator.NewGenerator(1, math.MaxUint8)
	upf.qerIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)
	upf.N3Interface = UPFInterfaceInfo{
		IPv4EndPointAddresses: make([]net.IP, 0),
	}

	return upf
}

func (upf *UPF) pdrID() (uint16, error) {
	pdrID, err := upf.pdrIDGenerator.Allocate()
	if err != nil {
		return 0, fmt.Errorf("could not allocate PDR ID: %v", err)
	}
	return uint16(pdrID), nil
}

func (upf *UPF) farID() (uint32, error) {
	var farID uint32
	if tmpID, err := upf.farIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		farID = uint32(tmpID)
	}

	return farID, nil
}

func (upf *UPF) barID() (uint8, error) {
	var barID uint8
	if tmpID, err := upf.barIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		barID = uint8(tmpID)
	}

	return barID, nil
}

func (upf *UPF) qerID() (uint32, error) {
	var qerID uint32
	if tmpID, err := upf.qerIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		qerID = uint32(tmpID)
	}

	return qerID, nil
}

func (upf *UPF) BuildCreatePdrFromPccRule(rule *models.PccRule) (*PDR, error) {
	var pdr *PDR
	var err error

	// create empty PDR
	if pdr, err = upf.AddPDR(); err != nil {
		return nil, err
	}

	// SDF Filter
	sdfFilter := SDFFilter{}

	// First Flow
	flow := rule.FlowInfos[0]

	// Flow Description
	if flow.FlowDescription != "" {
		sdfFilter.Fd = true
		sdfFilter.FlowDescription = []byte(flow.FlowDescription)
		sdfFilter.LengthOfFlowDescription = uint16(len(sdfFilter.FlowDescription))
		if id, err := strconv.ParseUint(flow.PackFiltID, 10, 32); err != nil {
			return nil, err
		} else {
			sdfFilter.SdfFilterID = uint32(id)
		}
	}

	// ToS Traffic Class
	if flow.TosTrafficClass != "" {
		sdfFilter.Ttc = true
		sdfFilter.TosTrafficClass = []byte(flow.TosTrafficClass)
	}

	// Flow Label
	if flow.FlowLabel != "" {
		sdfFilter.Fl = true
		sdfFilter.FlowLabel = []byte(flow.FlowLabel)
	}

	// Security Parameter Index
	if flow.Spi != "" {
		sdfFilter.Spi = true
		sdfFilter.SecurityParameterIndex = []byte(flow.Spi)
	}

	pdi := PDI{
		SDFFilter: &sdfFilter,
	}

	pdr.PDI = pdi
	pdr.Precedence = uint32(rule.Precedence)

	return pdr, nil
}

func (upf *UPF) AddPDR() (*PDR, error) {
	pdr := new(PDR)
	if PDRID, err := upf.pdrID(); err != nil {
		return nil, err
	} else {
		pdr.PDRID = PDRID
		upf.pdrPool.Store(pdr.PDRID, pdr)
	}

	if newFAR, err := upf.AddFAR(); err != nil {
		return nil, err
	} else {
		pdr.FAR = newFAR
	}

	return pdr, nil
}

func (upf *UPF) AddFAR() (*FAR, error) {
	far := new(FAR)
	// set default FAR action to drop
	far.ApplyAction.Drop = true
	if FARID, err := upf.farID(); err != nil {
		return nil, err
	} else {
		far.FARID = FARID
		upf.farPool.Store(far.FARID, far)
	}

	return far, nil
}

func (upf *UPF) AddBAR() (*BAR, error) {
	bar := new(BAR)
	if BARID, err := upf.barID(); err != nil {
		return nil, err
	} else {
		bar.BARID = BARID
		upf.barPool.Store(bar.BARID, bar)
	}

	return bar, nil
}

func (upf *UPF) AddQER() (*QER, error) {
	qer := new(QER)
	if QERID, err := upf.qerID(); err != nil {
		return nil, err
	} else {
		qer.QERID = QERID
		upf.qerPool.Store(qer.QERID, qer)
	}

	return qer, nil
}

func (upf *UPF) RemovePDR(pdr *PDR) {
	upf.pdrIDGenerator.FreeID(int64(pdr.PDRID))
	upf.pdrPool.Delete(pdr.PDRID)
}

func (upf *UPF) RemoveFAR(far *FAR) {
	upf.farIDGenerator.FreeID(int64(far.FARID))
	upf.farPool.Delete(far.FARID)
}

func (upf *UPF) RemoveBAR(bar *BAR) {
	upf.barIDGenerator.FreeID(int64(bar.BARID))
	upf.barPool.Delete(bar.BARID)
}

func (upf *UPF) RemoveQER(qer *QER) {
	upf.qerIDGenerator.FreeID(int64(qer.QERID))
	upf.qerPool.Delete(qer.QERID)
}

func GenerateDataPath(upf *UPF, smContext *SMContext) *DataPath {
	curDataPathNode := &DataPathNode{
		UpLinkTunnel:   &GTPTunnel{PDR: make(map[string]*PDR)},
		DownLinkTunnel: &GTPTunnel{PDR: make(map[string]*PDR)},
		UPF:            upf,
	}

	dataPath := &DataPath{
		DPNode: curDataPathNode,
	}
	return dataPath
}
