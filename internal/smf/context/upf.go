package context

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
)

// var upfPool sync.Map

type UPTunnel struct {
	PathIDGenerator *idgenerator.IDGenerator
	DataPathPool    DataPathPool
	ANInformation   struct {
		IPAddress net.IP
		TEID      uint32
	}
}

type UPFStatus int

const (
	NotAssociated          UPFStatus = 0
	AssociatedSettingUp    UPFStatus = 1
	AssociatedSetUpSuccess UPFStatus = 2
)

func (s UPFStatus) String() string {
	switch s {
	case NotAssociated:
		return "NotAssociated"
	case AssociatedSettingUp:
		return "AssociatedSettingUp"
	case AssociatedSetUpSuccess:
		return "AssociatedSetUpSuccess"
	default:
		return "invalid"
	}
}

type RecoveryTimeStamp struct {
	RecoveryTimeStamp time.Time
}

type UPF struct {
	SNssaiInfos        []SnssaiUPFInfo
	N3Interfaces       []UPFInterfaceInfo
	N9Interfaces       []UPFInterfaceInfo
	UPFunctionFeatures *UPFunctionFeatures

	pdrPool sync.Map
	farPool sync.Map
	barPool sync.Map
	qerPool sync.Map
	// urrPool        sync.Map
	pdrIDGenerator *idgenerator.IDGenerator
	farIDGenerator *idgenerator.IDGenerator
	barIDGenerator *idgenerator.IDGenerator
	qerIDGenerator *idgenerator.IDGenerator

	RecoveryTimeStamp RecoveryTimeStamp
	NodeID            NodeID
	UPFStatus         UPFStatus
	uuid              uuid.UUID
	Port              uint16
	NHeartBeat        uint8

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
func NewUPFInterfaceInfo(i *factory.InterfaceUpfInfoItem) *UPFInterfaceInfo {
	interfaceInfo := new(UPFInterfaceInfo)

	interfaceInfo.IPv4EndPointAddresses = make([]net.IP, 0)
	interfaceInfo.IPv6EndPointAddresses = make([]net.IP, 0)

	logger.CtxLog.Infoln("Endpoints:", i.Endpoints)

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

// *** add unit test ***//
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
			logger.CtxLog.Errorf("resolve addr [%s] failed", i.EndpointFQDN)
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

func (upfSelectionParams *UPFSelectionParams) String() string {
	str := ""
	Dnn := upfSelectionParams.Dnn
	if Dnn != "" {
		str += fmt.Sprintf("Dnn: %s\n", Dnn)
	}

	SNssai := upfSelectionParams.SNssai
	if SNssai != nil {
		str += fmt.Sprintf("Sst: %d, Sd: %s\n", int(SNssai.Sst), SNssai.Sd)
	}

	Dnai := upfSelectionParams.Dnai
	if Dnai != "" {
		str += fmt.Sprintf("DNAI: %s\n", Dnai)
	}

	return str
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
		logger.CtxLog.Warnf("Allocate pathID error: %+v", err)
		return
	}

	upTunnel.DataPathPool[pathID] = dataPath
}

// *** add unit test ***//
// NewUPF returns a new UPF context in SMF
func NewUPF(nodeID *NodeID, ifaces []factory.InterfaceUpfInfoItem) (upf *UPF) {
	upf = new(UPF)
	upf.uuid = uuid.New()

	// Initialize context
	upf.UPFStatus = NotAssociated
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
		case models.UpInterfaceType_N3:
			upf.N3Interfaces = append(upf.N3Interfaces, *upIface)
		case models.UpInterfaceType_N9:
			upf.N9Interfaces = append(upf.N9Interfaces, *upIface)
		}
	}

	return upf
}

// *** add unit test ***//
// GetInterface return the UPFInterfaceInfo that match input cond
func (upf *UPF) GetInterface(interfaceType models.UpInterfaceType, dnn string) *UPFInterfaceInfo {
	switch interfaceType {
	case models.UpInterfaceType_N3:
		for i, iface := range upf.N3Interfaces {
			if iface.NetworkInstance == dnn {
				return &upf.N3Interfaces[i]
			}
		}
	case models.UpInterfaceType_N9:
		for i, iface := range upf.N9Interfaces {
			if iface.NetworkInstance == dnn {
				return &upf.N9Interfaces[i]
			}
		}
	}
	return nil
}

func (upf *UPF) PFCPAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   upf.NodeID.ResolveNodeIdToIp(),
		Port: factory.UPF_PFCP_PORT,
	}
}

func (upf *UPF) pdrID() (uint16, error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf not associate with smf")
		return 0, err
	}

	var pdrID uint16
	if tmpID, err := upf.pdrIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		pdrID = uint16(tmpID)
	}

	return pdrID, nil
}

func (upf *UPF) farID() (uint32, error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf not associate with smf")
		return 0, err
	}

	var farID uint32
	if tmpID, err := upf.farIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		farID = uint32(tmpID)
	}

	return farID, nil
}

func (upf *UPF) barID() (uint8, error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf not associate with smf")
		return 0, err
	}

	var barID uint8
	if tmpID, err := upf.barIDGenerator.Allocate(); err != nil {
		return 0, err
	} else {
		barID = uint8(tmpID)
	}

	return barID, nil
}

func (upf *UPF) qerID() (uint32, error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf not associate with smf")
		return 0, err
	}

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
		if id, err := strconv.ParseUint(flow.PackFiltId, 10, 32); err != nil {
			return nil, err
		} else {
			sdfFilter.SdfFilterId = uint32(id)
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
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf do not associate with smf")
		return nil, err
	}

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
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf do not associate with smf")
		return nil, err
	}

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
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf do not associate with smf")
		return nil, err
	}

	bar := new(BAR)
	if BARID, err := upf.barID(); err != nil {
	} else {
		bar.BARID = BARID
		upf.barPool.Store(bar.BARID, bar)
	}

	return bar, nil
}

func (upf *UPF) AddQER() (*QER, error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err := fmt.Errorf("this upf do not associate with smf")
		return nil, err
	}

	qer := new(QER)
	if QERID, err := upf.qerID(); err != nil {
	} else {
		qer.QERID = QERID
		upf.qerPool.Store(qer.QERID, qer)
	}

	return qer, nil
}

// *** add unit test ***//
func (upf *UPF) RemovePDR(pdr *PDR) (err error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err = fmt.Errorf("this upf not associate with smf")
		return err
	}

	upf.pdrIDGenerator.FreeID(int64(pdr.PDRID))
	upf.pdrPool.Delete(pdr.PDRID)
	return nil
}

// *** add unit test ***//
func (upf *UPF) RemoveFAR(far *FAR) (err error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err = fmt.Errorf("this upf not associate with smf")
		return err
	}

	upf.farIDGenerator.FreeID(int64(far.FARID))
	upf.farPool.Delete(far.FARID)
	return nil
}

// *** add unit test ***//
func (upf *UPF) RemoveBAR(bar *BAR) (err error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err = fmt.Errorf("this upf not associate with smf")
		return err
	}

	upf.barIDGenerator.FreeID(int64(bar.BARID))
	upf.barPool.Delete(bar.BARID)
	return nil
}

// *** add unit test ***//
func (upf *UPF) RemoveQER(qer *QER) (err error) {
	if upf.UPFStatus != AssociatedSetUpSuccess {
		err = fmt.Errorf("this upf not associate with smf")
		return err
	}

	upf.qerIDGenerator.FreeID(int64(qer.QERID))
	upf.qerPool.Delete(qer.QERID)
	return nil
}

func (upf *UPF) isSupportSnssai(snssai *SNssai) bool {
	for _, snssaiInfo := range upf.SNssaiInfos {
		if snssaiInfo.SNssai.Equal(snssai) {
			return true
		}
	}
	return false
}

// IsUpfSupportUeIpAddrAlloc UE IP addr alloc by UPF supported
func (upf *UPF) IsUpfSupportUeIpAddrAlloc() bool {
	if upf.UPFunctionFeatures != nil &&
		(upf.UPFunctionFeatures.SupportedFeatures1&UpFunctionFeatures1Ueip) == UpFunctionFeatures1Ueip {
		return true
	}

	return false
}
