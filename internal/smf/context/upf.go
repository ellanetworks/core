// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"errors"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/google/uuid"
	"github.com/omec-project/nas/nasMessage"
)

type UPTunnel struct {
	PathIDGenerator *idgenerator.IDGenerator
	DataPathPool    DataPathPool
	ANInformation   struct {
		IPAddress net.IP
		TEID      uint32
	}
}

type UPFStatus int

type RecoveryTimeStamp struct {
	RecoveryTimeStamp time.Time
}

type UPF struct {
	SNssaiInfos  []SnssaiUPFInfo
	N3Interfaces []UPFInterfaceInfo
	N9Interfaces []UPFInterfaceInfo

	pdrPool sync.Map
	farPool sync.Map
	barPool sync.Map
	qerPool sync.Map

	pdrIDGenerator *idgenerator.IDGenerator
	farIDGenerator *idgenerator.IDGenerator
	barIDGenerator *idgenerator.IDGenerator
	qerIDGenerator *idgenerator.IDGenerator

	NodeID NodeID
	uuid   uuid.UUID

	// lock
	UpfLock sync.RWMutex
}

// UPFSelectionParams ... parameters for upf selection
type UPFSelectionParams struct {
	Dnn    string
	SNssai *SNssai
	Dnai   string
}

// UPFInterfaceInfo store the UPF interface information
type UPFInterfaceInfo struct {
	NetworkInstance       string
	EndpointFQDN          string
	IPv4EndPointAddresses []net.IP
	IPv6EndPointAddresses []net.IP
}

// NewUPFInterfaceInfo parse the InterfaceUpfInfoItem to generate UPFInterfaceInfo
func NewUPFInterfaceInfo(i *InterfaceUpfInfoItem) *UPFInterfaceInfo {
	interfaceInfo := new(UPFInterfaceInfo)

	interfaceInfo.IPv4EndPointAddresses = make([]net.IP, 0)
	interfaceInfo.IPv6EndPointAddresses = make([]net.IP, 0)

	for _, endpoint := range i.Endpoints {
		eIP := net.ParseIP(endpoint)
		if eIP == nil {
			interfaceInfo.EndpointFQDN = endpoint
		} else if eIPv4 := eIP.To4(); eIPv4 == nil {
			interfaceInfo.IPv6EndPointAddresses = append(interfaceInfo.IPv6EndPointAddresses, eIP)
		} else {
			interfaceInfo.IPv4EndPointAddresses = append(interfaceInfo.IPv4EndPointAddresses, eIPv4)
		}
	}

	interfaceInfo.NetworkInstance = i.NetworkInstance

	return interfaceInfo
}

// IP returns the IP of the user plane IP information of the pduSessType
func (i *UPFInterfaceInfo) IP(pduSessType uint8) (net.IP, error) {
	if (pduSessType == nasMessage.PDUSessionTypeIPv4 || pduSessType == nasMessage.PDUSessionTypeIPv4IPv6) && len(i.IPv4EndPointAddresses) != 0 {
		return i.IPv4EndPointAddresses[0].To4(), nil
	}

	if (pduSessType == nasMessage.PDUSessionTypeIPv6 || pduSessType == nasMessage.PDUSessionTypeIPv4IPv6) && len(i.IPv6EndPointAddresses) != 0 {
		return i.IPv6EndPointAddresses[0], nil
	}

	if i.EndpointFQDN != "" {
		if resolvedAddr, err := net.ResolveIPAddr("ip", i.EndpointFQDN); err != nil {
			logger.SmfLog.Errorf("resolve addr [%s] failed", i.EndpointFQDN)
		} else {
			if pduSessType == nasMessage.PDUSessionTypeIPv4 {
				return resolvedAddr.IP.To4(), nil
			} else if pduSessType == nasMessage.PDUSessionTypeIPv6 {
				return resolvedAddr.IP.To16(), nil
			} else {
				v4addr := resolvedAddr.IP.To4()
				if v4addr != nil {
					return v4addr, nil
				} else {
					return resolvedAddr.IP.To16(), nil
				}
			}
		}
	}

	return nil, errors.New("not matched ip address")
}

// UUID return this UPF UUID (allocate by SMF in this time)
// Maybe allocate by UPF in future
func (upf *UPF) UUID() string {
	uuid := upf.uuid.String()
	return uuid
}

func NewUPTunnel() (tunnel *UPTunnel) {
	tunnel = &UPTunnel{
		DataPathPool:    make(DataPathPool),
		PathIDGenerator: idgenerator.NewGenerator(1, 2147483647),
	}

	return
}

// *** add unit test ***//
func (upTunnel *UPTunnel) AddDataPath(dataPath *DataPath) {
	pathID, err := upTunnel.PathIDGenerator.Allocate()
	if err != nil {
		logger.SmfLog.Warnf("Allocate pathID error: %+v", err)
		return
	}

	upTunnel.DataPathPool[pathID] = dataPath
}

// *** add unit test ***//
// NewUPF returns a new UPF context in SMF
func NewUPF(nodeID *NodeID, ifaces []InterfaceUpfInfoItem) (upf *UPF) {
	upf = new(UPF)
	upf.uuid = uuid.New()

	// Initialize context
	upf.NodeID = *nodeID
	upf.pdrIDGenerator = idgenerator.NewGenerator(1, math.MaxUint16)
	upf.farIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)
	upf.barIDGenerator = idgenerator.NewGenerator(1, math.MaxUint8)
	upf.qerIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)

	upf.N3Interfaces = make([]UPFInterfaceInfo, 0)
	upf.N9Interfaces = make([]UPFInterfaceInfo, 0)

	for _, iface := range ifaces {
		upIface := NewUPFInterfaceInfo(&iface)

		switch iface.InterfaceType {
		case models.UpInterfaceTypeN3:
			upf.N3Interfaces = append(upf.N3Interfaces, *upIface)
		case models.UpInterfaceTypeN9:
			upf.N9Interfaces = append(upf.N9Interfaces, *upIface)
		}
	}

	return upf
}

// GetInterface return the UPFInterfaceInfo that match input cond
func (upf *UPF) GetInterface(interfaceType models.UpInterfaceType, dnn string) *UPFInterfaceInfo {
	switch interfaceType {
	case models.UpInterfaceTypeN3:
		for i, iface := range upf.N3Interfaces {
			if iface.NetworkInstance == dnn {
				return &upf.N3Interfaces[i]
			}
		}
	case models.UpInterfaceTypeN9:
		for i, iface := range upf.N9Interfaces {
			if iface.NetworkInstance == dnn {
				return &upf.N9Interfaces[i]
			}
		}
	}
	return nil
}

func (upf *UPF) pdrID() (uint16, error) {
	var pdrID uint16
	if tmpID, err := upf.pdrIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		pdrID = uint16(tmpID)
	}

	return pdrID, nil
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
