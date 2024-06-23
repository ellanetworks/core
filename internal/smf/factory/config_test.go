package factory

import (
	"fmt"
	"testing"

	"github.com/omec-project/openapi/models"
	"github.com/stretchr/testify/assert"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
)

const (
	GNB = "gnb"
)

func TestUpdateSliceInfo(t *testing.T) {
	cfg1 := Configuration{}
	cfg2 := Configuration{}

	cfg1.parseRocConfig(makeDummyConfig("1", "010203"))
	cfg2.parseRocConfig(makeDummyConfig("2", "010203"))

	compareAndProcessConfigs(&cfg1, &cfg2)
}

func makeDummyConfig(sst, sd string) *protos.NetworkSliceResponse {
	var rsp protos.NetworkSliceResponse

	rsp.NetworkSlice = make([]*protos.NetworkSlice, 0)

	ns := protos.NetworkSlice{Name: "Enterprise-1"}
	slice := protos.NSSAI{Sst: sst, Sd: sd}
	ns.Nssai = &slice

	upf := protos.UpfInfo{UpfName: "upf", UpfPort: 8805}
	site := protos.SiteInfo{SiteName: "siteOne", Upf: &upf, Gnb: make([]*protos.GNodeB, 0)}
	gNb := protos.GNodeB{Name: "gnb"}
	site.Gnb = append(site.Gnb, &gNb)
	ns.Site = &site

	ns.DeviceGroup = make([]*protos.DeviceGroup, 0)
	ipDomain := protos.IpDomain{DnnName: "internet", UePool: "60.60.0.0/16", DnsPrimary: "8.8.8.8", Mtu: 1400}
	devGrp := protos.DeviceGroup{IpDomainDetails: &ipDomain}
	ns.DeviceGroup = append(ns.DeviceGroup, &devGrp)

	rsp.NetworkSlice = append(rsp.NetworkSlice, &ns)
	return &rsp
}

func TestCompareSliceConfigIdentical(t *testing.T) {
	sNssai1 := models.Snssai{Sst: 1, Sd: "010203"}
	sNssai2 := models.Snssai{Sst: 1, Sd: "010203"}

	dnnInfo1 := SnssaiDnnInfoItem{Dnn: "DNN1", UESubnet: "10.10.0.0/16", DNS: DNS{IPv4Addr: "1.1.1.1"}}
	dnnInfo2 := SnssaiDnnInfoItem{Dnn: "DNN2", UESubnet: "10.10.0.0/16", DNS: DNS{IPv4Addr: "1.1.1.1"}}
	dnnInfo3 := SnssaiDnnInfoItem{Dnn: "DNN1", UESubnet: "10.10.0.0/16", DNS: DNS{IPv4Addr: "1.1.1.1"}}
	dnnInfo4 := SnssaiDnnInfoItem{Dnn: "DNN2", UESubnet: "10.10.0.0/16", DNS: DNS{IPv4Addr: "1.1.1.1"}}

	sNssaiInfoItem1 := SnssaiInfoItem{SNssai: &sNssai1, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	sNssaiInfoItem1.DnnInfos = append(sNssaiInfoItem1.DnnInfos, dnnInfo1, dnnInfo2)
	slice1 := []SnssaiInfoItem{sNssaiInfoItem1}

	sNssaiInfoItem2 := SnssaiInfoItem{SNssai: &sNssai2, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	sNssaiInfoItem2.DnnInfos = append(sNssaiInfoItem2.DnnInfos, dnnInfo4, dnnInfo3)
	slice2 := []SnssaiInfoItem{sNssaiInfoItem2}

	match, add, mod, del := compareNetworkSlices(slice1, slice2)

	if !match {
		t.Errorf("Expected NetworkSlice configurations to be different, but they were identical")
	}

	if len(add) != 0 {
		t.Errorf("Expected 0 NetworkSlices to be added, but got %d", len(add))
	}

	if len(mod) != 0 {
		t.Errorf("Expected 0 NetworkSlices to be modified, but got %d", len(mod))
	}

	if len(del) != 0 {
		t.Errorf("Expected 0 NetworkSlices to be deleted, but got %d", len(del))
	}
}

func TestCompareSliceConfigDifferent(t *testing.T) {
	sNssai1 := models.Snssai{Sst: 1, Sd: "010203"}
	sNssai2 := models.Snssai{Sst: 1, Sd: "010204"}

	dnnInfo1 := SnssaiDnnInfoItem{Dnn: "DNN1", UESubnet: "11.11.0.0/16", DNS: DNS{IPv4Addr: "1.1.1.1"}}
	dnnInfo2 := SnssaiDnnInfoItem{Dnn: "DNN2", UESubnet: "12.12.0.0/16", DNS: DNS{IPv4Addr: "2.2.2.2"}}
	dnnInfo3 := SnssaiDnnInfoItem{Dnn: "DNN3", UESubnet: "13.13.0.0/16", DNS: DNS{IPv4Addr: "3.3.3.3"}}
	dnnInfo4 := SnssaiDnnInfoItem{Dnn: "DNN4", UESubnet: "14.14.0.0/16", DNS: DNS{IPv4Addr: "4.4.4.4"}}

	sNssaiInfoItem1 := SnssaiInfoItem{SNssai: &sNssai1, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	sNssaiInfoItem1.DnnInfos = append(sNssaiInfoItem1.DnnInfos, dnnInfo1, dnnInfo2)
	slice1 := []SnssaiInfoItem{sNssaiInfoItem1}

	sNssaiInfoItem2 := SnssaiInfoItem{SNssai: &sNssai2, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	sNssaiInfoItem2.DnnInfos = append(sNssaiInfoItem2.DnnInfos, dnnInfo4, dnnInfo3)
	slice2 := []SnssaiInfoItem{sNssaiInfoItem2}

	match, add, mod, del := compareNetworkSlices(slice1, slice2)

	if match {
		t.Errorf("Expected NetworkSlice configurations to be different, but they were identical")
	}

	if len(add) != 1 {
		t.Errorf("Expected 1 NetworkSlice to be added, but got %d", len(add))
	}

	if len(mod) != 0 {
		t.Errorf("Expected 1 NetworkSlice to be modified, but got %d", len(mod))
	}

	if len(del) != 1 {
		t.Errorf("Expected 1 NetworkSlice to be deleted, but got %d", len(del))
	}
}

func TestCompareSliceConfigModified(t *testing.T) {
	sNssai1 := models.Snssai{Sst: 1, Sd: "010203"}
	sNssai2 := models.Snssai{Sst: 1, Sd: "010203"}

	dnnInfo := SnssaiDnnInfoItem{Dnn: "DNN1", UESubnet: "11.11.0.0/16", DNS: DNS{IPv4Addr: "3.3.3.3"}}

	sNssaiInfoItem1 := SnssaiInfoItem{SNssai: &sNssai1, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	slice1 := []SnssaiInfoItem{sNssaiInfoItem1}

	sNssaiInfoItem2 := SnssaiInfoItem{SNssai: &sNssai2, DnnInfos: make([]SnssaiDnnInfoItem, 0)}
	sNssaiInfoItem2.DnnInfos = append(sNssaiInfoItem2.DnnInfos, dnnInfo)
	slice2 := []SnssaiInfoItem{sNssaiInfoItem2}

	match, add, mod, del := compareNetworkSlices(slice1, slice2)

	if match {
		t.Errorf("Expected NetworkSlice configurations to be different, but they were identical")
	}

	if len(add) != 0 {
		t.Errorf("Expected 0 NetworkSlices to be added, but got %d", len(add))
	}

	if len(mod) != 1 {
		t.Errorf("Expected 1 NetworkSlice to be modified, but got %d", len(mod))
	}

	if len(del) != 0 {
		t.Errorf("Expected 0 NetworkSlices to be deleted, but got %d", len(del))
	}
}

func TestCompareUPNodesConfigs(t *testing.T) {
	u1 := UPNode{
		Type:                 "UPF",
		NodeID:               "u1.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0), //[]models.SnssaiUpfInfoItem `yaml:"sNssaiUpfInfos,omitempty"`
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),     //[]InterfaceUpfInfoItem,
	}
	slice1 := models.Snssai{Sst: 1, Sd: "010203"}
	slice2 := models.Snssai{Sst: 2, Sd: "020203"}
	sn1 := models.SnssaiUpfInfoItem{SNssai: &slice1, DnnUpfInfoList: make([]models.DnnUpfInfoItem, 0)}
	sn2 := models.SnssaiUpfInfoItem{SNssai: &slice2, DnnUpfInfoList: make([]models.DnnUpfInfoItem, 0)}
	dnn1 := models.DnnUpfInfoItem{Dnn: "DNN1"}
	dnn11 := models.DnnUpfInfoItem{Dnn: "DNN11"}
	dnn2 := models.DnnUpfInfoItem{Dnn: "DNN2"}
	dnn21 := models.DnnUpfInfoItem{Dnn: "DNN21"}

	sn1.DnnUpfInfoList = append(sn1.DnnUpfInfoList, dnn1, dnn11)
	sn2.DnnUpfInfoList = append(sn2.DnnUpfInfoList, dnn2, dnn21)
	u1.SNssaiInfos = []models.SnssaiUpfInfoItem{sn1, sn2}

	u2 := UPNode{
		Type:                 "UPF",
		NodeID:               "u2.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0), //[]models.SnssaiUpfInfoItem `yaml:"sNssaiUpfInfos,omitempty"`
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),     //[]InterfaceUpfInfoItem,
	}
	slice3 := models.Snssai{Sst: 1, Sd: "010203"}
	slice4 := models.Snssai{Sst: 2, Sd: "020203"}
	sn3 := models.SnssaiUpfInfoItem{SNssai: &slice3, DnnUpfInfoList: make([]models.DnnUpfInfoItem, 0)}
	sn4 := models.SnssaiUpfInfoItem{SNssai: &slice4, DnnUpfInfoList: make([]models.DnnUpfInfoItem, 0)}
	dnn3 := models.DnnUpfInfoItem{Dnn: "DNN1"}
	dnn31 := models.DnnUpfInfoItem{Dnn: "DNN11"}
	dnn4 := models.DnnUpfInfoItem{Dnn: "DNN2"}
	dnn41 := models.DnnUpfInfoItem{Dnn: "DNN21"}

	sn1.DnnUpfInfoList = append(sn1.DnnUpfInfoList, dnn3, dnn31)
	sn2.DnnUpfInfoList = append(sn2.DnnUpfInfoList, dnn4, dnn41)
	u2.SNssaiInfos = []models.SnssaiUpfInfoItem{sn3, sn4}

	up1, up2 := make(map[string]UPNode), make(map[string]UPNode)

	up1["u1"] = u1
	up2["u2"] = u2
	match, add, mod, del := compareUPNodesConfigs(up1, up2)

	if match {
		t.Errorf("Expected UPNode configurations to be different, but they were identical")
	}

	if len(add) != 1 {
		t.Errorf("Expected 1 UPNode to be added, but got %d", len(add))
	}

	if len(mod) != 0 {
		t.Errorf("Expected 0 UPNodes to be modified, but got %d", len(mod))
	}

	if len(del) != 1 {
		t.Errorf("Expected 1 UPNode to be deleted, but got %d", len(del))
	}
}

func TestCompareUPNodesConfigsIdentical(t *testing.T) {
	u1 := UPNode{
		Type:                 "UPF",
		NodeID:               "u1.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0),
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),
	}
	u2 := UPNode{
		Type:                 "UPF",
		NodeID:               "u1.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0),
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),
	}

	snssai1 := models.Snssai{Sst: 1, Sd: "010203"}
	dnn1 := models.DnnUpfInfoItem{Dnn: "DNN1"}
	snssai2 := models.Snssai{Sst: 1, Sd: "010203"}
	dnn2 := models.DnnUpfInfoItem{Dnn: "DNN1"}

	snssaiInfoItem1 := models.SnssaiUpfInfoItem{SNssai: &snssai1, DnnUpfInfoList: []models.DnnUpfInfoItem{dnn1}}
	snssaiInfoItem2 := models.SnssaiUpfInfoItem{SNssai: &snssai2, DnnUpfInfoList: []models.DnnUpfInfoItem{dnn2}}
	u1.SNssaiInfos = append(u1.SNssaiInfos, snssaiInfoItem1)
	u2.SNssaiInfos = append(u2.SNssaiInfos, snssaiInfoItem2)

	up1, up2 := make(map[string]UPNode), make(map[string]UPNode)
	up1["u1"] = u1
	up2["u1"] = u2

	match, add, mod, del := compareUPNodesConfigs(up1, up2)

	if !match {
		t.Errorf("Expected UPNode configurations to be identical, but they were not")
	}

	if len(add) != 0 {
		t.Errorf("Expected 0 UPNodes to be added, but got %d", len(add))
	}

	if len(mod) != 0 {
		t.Errorf("Expected 0 UPNodes to be modified, but got %d", len(mod))
	}

	if len(del) != 0 {
		t.Errorf("Expected 0 UPNodes to be deleted, but got %d", len(del))
	}
}

func TestCompareUPNodesConfigsDifferentDNN(t *testing.T) {
	u1 := UPNode{
		Type:                 "UPF",
		NodeID:               "u1.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0),
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),
	}
	u2 := UPNode{
		Type:                 "UPF",
		NodeID:               "u1.abc.def.com",
		SNssaiInfos:          make([]models.SnssaiUpfInfoItem, 0),
		InterfaceUpfInfoList: make([]InterfaceUpfInfoItem, 0),
	}

	snssai1 := models.Snssai{Sst: 1, Sd: "010203"}
	dnn1 := models.DnnUpfInfoItem{Dnn: "DNN1"}
	snssai2 := models.Snssai{Sst: 1, Sd: "010203"}
	dnn2 := models.DnnUpfInfoItem{Dnn: "DNN2"}

	snssaiInfoItem1 := models.SnssaiUpfInfoItem{SNssai: &snssai1, DnnUpfInfoList: []models.DnnUpfInfoItem{dnn1}}
	snssaiInfoItem2 := models.SnssaiUpfInfoItem{SNssai: &snssai2, DnnUpfInfoList: []models.DnnUpfInfoItem{dnn2}}
	u1.SNssaiInfos = append(u1.SNssaiInfos, snssaiInfoItem1)
	u2.SNssaiInfos = append(u2.SNssaiInfos, snssaiInfoItem2)

	up1, up2 := make(map[string]UPNode), make(map[string]UPNode)
	up1["u1"] = u1
	up2["u1"] = u2

	match, add, mod, del := compareUPNodesConfigs(up1, up2)

	if match {
		t.Errorf("Expected UPNode configurations to be different, but they were identical")
	}

	if len(add) != 0 {
		t.Errorf("Expected 0 UPNodes to be added, but got %d", len(add))
	}

	if len(mod) != 1 {
		t.Errorf("Expected 1 UPNode to be modified, but got %d", len(mod))
	}

	if len(del) != 0 {
		t.Errorf("Expected 0 UPNodes to be deleted, but got %d", len(del))
	}
}

func TestCompareGenericSlicesDifferent1(t *testing.T) {
	l1 := UPLink{A: "gnb", B: "upf1"}
	l2 := UPLink{A: "gnb", B: "upf2"}
	l3 := UPLink{A: "gnb", B: "upf3"}
	l4 := UPLink{A: "gnb", B: "upf4"}

	match, addLinksInterface, delLinksInterface := compareGenericSlices([]UPLink{l1, l2}, []UPLink{l3, l4}, compareUPLinks)

	if match {
		t.Errorf("Expected GenericSlice configurations to be different, but they were identical")
	}

	addLinks, ok := addLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected addLinks to be of type []UPLink, but it was not")
	}

	if len(addLinks) != 2 {
		t.Errorf("Expected 2 GenericSlices to be added, but got %d", len(addLinks))
	}
	if addLinks[0].A != GNB || addLinks[0].B != "upf3" {
		t.Errorf("Expected GenericSlice to be added, but got %v", addLinks[0])
	}
	if addLinks[1].A != GNB || addLinks[1].B != "upf4" {
		t.Errorf("Expected GenericSlice to be added, but got %v", addLinks[1])
	}

	delLinks, ok := delLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected delLinks to be of type []UPLink, but it was not")
	}

	if len(delLinks) != 2 {
		t.Errorf("Expected 2 GenericSlices to be deleted, but got %d", len(delLinks))
	}
	if delLinks[0].A != GNB || delLinks[0].B != "upf1" {
		t.Errorf("Expected GenericSlice to be deleted, but got %v", delLinks[0])
	}
	if delLinks[1].A != GNB || delLinks[1].B != "upf2" {
		t.Errorf("Expected GenericSlice to be deleted, but got %v", delLinks[1])
	}
}

func TestCompareGenericSlicesDifferent2(t *testing.T) {
	l1 := UPLink{A: "gnb", B: "upf1"}

	match, addLinksInterface, delLinksInterface := compareGenericSlices([]UPLink{}, []UPLink{l1}, compareUPLinks)

	if match {
		t.Errorf("Expected GenericSlice configurations to be different, but they were identical")
	}

	addLinks, ok := addLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected addLinks to be of type []UPLink, but it was not")
	}

	if len(addLinks) != 1 {
		t.Errorf("Expected 2 GenericSlices to be added, but got %d", len(addLinks))
	}
	if addLinks[0].A != GNB || addLinks[0].B != "upf1" {
		t.Errorf("Expected GenericSlice to be added, but got %v", addLinks[0])
	}

	delLinks, ok := delLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected delLinks to be of type []UPLink, but it was not")
	}

	if len(delLinks) != 0 {
		t.Errorf("Expected 0 GenericSlices to be deleted, but got %d", len(delLinks))
	}
}

func TestCompareGenericSlicesIdentical(t *testing.T) {
	l1 := UPLink{A: "gnb", B: "upf1"}
	l2 := UPLink{A: "gnb", B: "upf2"}
	l3 := UPLink{A: "gnb", B: "upf1"}
	l4 := UPLink{A: "gnb", B: "upf2"}

	match, addLinksInterface, delLinksInterface := compareGenericSlices([]UPLink{l1, l2}, []UPLink{l3, l4}, compareUPLinks)

	if !match {
		t.Errorf("Expected GenericSlice configurations to be identical, but they were not")
	}

	addLinks, ok := addLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected addLinks to be of type []UPLink, but it was not")
	}

	if len(addLinks) != 0 {
		t.Errorf("Expected 0 GenericSlices to be added, but got %d", len(addLinks))
	}

	delLinks, ok := delLinksInterface.([]UPLink)

	if !ok {
		t.Fatalf("Expected delLinks to be of type []UPLink, but it was not")
	}

	if len(delLinks) != 0 {
		t.Errorf("Expected 0 GenericSlices to be deleted, but got %d", len(delLinks))
	}
}

func TestKafkaEnabledByDefault(t *testing.T) {
	err := InitConfigFactory("../config/smfcfg.yaml")
	if err != nil {
		t.Errorf("Could not load default configuration file: %v", err)
	}
	if *SmfConfig.Configuration.KafkaInfo.EnableKafka != true {
		t.Errorf("Expected Kafka to be enabled by default, was disabled")
	}
}

// Webui URL is not set then default Webui URL value is returned
func TestGetDefaultWebuiUrl(t *testing.T) {
	if err := InitConfigFactory("../config/smfcfg.yaml"); err != nil {
		fmt.Printf("Error in InitConfigFactory: %v\n", err)
	}
	got := SmfConfig.Configuration.WebuiUri
	want := "webui:9876"
	assert.Equal(t, got, want, "The webui URL is not correct.")
}

// Webui URL is set to a custom value then custom Webui URL is returned
func TestGetCustomWebuiUrl(t *testing.T) {
	if err := InitConfigFactory("../config/smfcfg_with_custom_webui_url.yaml"); err != nil {
		fmt.Printf("Error in InitConfigFactory: %v\n", err)
	}
	got := SmfConfig.Configuration.WebuiUri
	want := "myspecialwebui:9872"
	assert.Equal(t, got, want, "The webui URL is not correct.")
}
