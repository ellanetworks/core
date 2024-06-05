package producer

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"time"

	"github.com/antihax/optional"
	"github.com/bronze1man/radius"

	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	Nudm_UEAU "github.com/omec-project/openapi/Nudm_UEAuthentication"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/ausf/consumer"
	ausf_context "github.com/yeastengine/canard/internal/ausf/context"
	"github.com/yeastengine/canard/internal/ausf/logger"
)

func KDF5gAka(param ...string) hash.Hash {
	s := param[0]
	s += param[1]
	if p0len, err := strconv.Atoi(param[2]); err != nil {
		logger.EapAuthComfirmLog.Warnf("atoi failed: %+v", err)
	} else {
		s += strconv.FormatInt(int64(p0len), 16)
	}
	h := hmac.New(sha256.New, []byte(s))

	return h
}

func intToByteArray(i int) []byte {
	r := make([]byte, 2)
	binary.BigEndian.PutUint16(r, uint16(i))
	return r
}

func padZeros(byteArray []byte, size int) []byte {
	l := len(byteArray)
	if l == size {
		return byteArray
	}
	r := make([]byte, size)
	copy(r[size-l:], byteArray)
	return r
}

func CalculateAtMAC(key []byte, input []byte) []byte {
	// keyed with K_aut
	h := hmac.New(sha256.New, key)
	if _, err := h.Write(input); err != nil {
		logger.EapAuthComfirmLog.Errorln(err.Error())
	}
	sha := string(h.Sum(nil))
	return []byte(sha[:16])
}

// func EapEncodeAttribute(attributeType string, data string) (returnStr string, err error) {
func EapEncodeAttribute(attributeType string, data string) (string, error) {
	var attribute string
	var length int

	switch attributeType {
	case "AT_RAND":
		length = len(data)/8 + 1
		if length != 5 {
			return "", fmt.Errorf("[eapEncodeAttribute] AT_RAND Length Error")
		}
		attrNum := fmt.Sprintf("%02x", ausf_context.AT_RAND_ATTRIBUTE)
		attribute = attrNum + "05" + "0000" + data

	case "AT_AUTN":
		length = len(data)/8 + 1
		if length != 5 {
			return "", fmt.Errorf("[eapEncodeAttribute] AT_AUTN Length Error")
		}
		attrNum := fmt.Sprintf("%02x", ausf_context.AT_AUTN_ATTRIBUTE)
		attribute = attrNum + "05" + "0000" + data

	case "AT_KDF_INPUT":
		var byteName []byte
		nLength := len(data)
		length := (nLength+3)/4 + 1
		b := make([]byte, length*4)
		byteNameLength := intToByteArray(nLength)
		byteName = []byte(data)
		pad := padZeros(byteName, (length-1)*4)
		b[0] = 23
		b[1] = byte(length)
		copy(b[2:4], byteNameLength)
		copy(b[4:], pad)
		return string(b[:]), nil

	case "AT_KDF":
		// Value 1 default key derivation function for EAP-AKA'
		attrNum := fmt.Sprintf("%02x", ausf_context.AT_KDF_ATTRIBUTE)
		attribute = attrNum + "01" + "0001"

	case "AT_MAC":
		// Pad MAC value with 16 bytes of 0 since this is just for the calculation of MAC
		attrNum := fmt.Sprintf("%02x", ausf_context.AT_MAC_ATTRIBUTE)
		attribute = attrNum + "05" + "0000" + "00000000000000000000000000000000"

	case "AT_RES":
		var byteName []byte
		nLength := len(data)
		length := (nLength+3)/4 + 1
		b := make([]byte, length*4)
		byteNameLength := intToByteArray(nLength)
		byteName = []byte(data)
		pad := padZeros(byteName, (length-1)*4)
		b[0] = 3
		b[1] = byte(length)
		copy(b[2:4], byteNameLength)
		copy(b[4:], pad)
		return string(b[:]), nil

	default:
		logger.EapAuthComfirmLog.Errorf("UNKNOWN attributeType %s\n", attributeType)
		return "", nil
	}

	if r, err := hex.DecodeString(attribute); err != nil {
		return "", err
	} else {
		return string(r), nil
	}
}

// func eapAkaPrimePrf(ikPrime string, ckPrime string, identity string) (K_encr string, K_aut string, K_re string,
//
//	MSK string, EMSK string) {
func eapAkaPrimePrf(ikPrime string, ckPrime string, identity string) (string, string, string, string, string) {
	keyAp := ikPrime + ckPrime

	var key []byte
	if keyTmp, err := hex.DecodeString(keyAp); err != nil {
		logger.EapAuthComfirmLog.Warnf("Decode key AP failed: %+v", err)
	} else {
		key = keyTmp
	}
	sBase := []byte("EAP-AKA'" + identity)

	MK := ""
	prev := []byte("")
	//_ = prev
	prfRounds := 208/32 + 1
	for i := 0; i < prfRounds; i++ {
		// Create a new HMAC by defining the hash type and the key (as byte array)
		h := hmac.New(sha256.New, key)

		hexNum := fmt.Sprint(i + 1)
		ap := append(sBase, hexNum...)
		s := append(prev, ap...)

		// Write Data to it
		if _, err := h.Write(s); err != nil {
			logger.EapAuthComfirmLog.Errorln(err.Error())
		}

		// Get result and encode as hexadecimal string
		sha := string(h.Sum(nil))
		MK += sha
		prev = []byte(sha)
	}

	K_encr := MK[0:16]  // 0..127
	K_aut := MK[16:48]  // 128..383
	K_re := MK[48:80]   // 384..639
	MSK := MK[80:144]   // 640..1151
	EMSK := MK[144:208] // 1152..1663
	return K_encr, K_aut, K_re, MSK, EMSK
}

func checkMACintegrity(offset int, expectedMacValue []byte, packet []byte, Kautn string) bool {
	eapDecode, decodeErr := radius.EapDecode(packet)
	if decodeErr != nil {
		logger.EapAuthComfirmLog.Infoln(decodeErr.Error())
	}
	if zeroBytes, err := hex.DecodeString("00000000000000000000000000000000"); err != nil {
		logger.EapAuthComfirmLog.Warnf("Decode error: %+v", err)
	} else {
		copy(eapDecode.Data[offset+4:offset+20], zeroBytes)
	}
	encodeAfter := eapDecode.Encode()
	MACvalue := CalculateAtMAC([]byte(Kautn), encodeAfter)

	if bytes.Equal(MACvalue, expectedMacValue) {
		return true
	} else {
		return false
	}
}

// func decodeResMac(packetData []byte, wholePacket []byte, Kautn string) (RES []byte, success bool) {
func decodeResMac(packetData []byte, wholePacket []byte, Kautn string) ([]byte, bool) {
	detectRes := false
	detectMac := false
	macCorrect := false
	dataArray := packetData
	var attributeLength int
	var attributeType int
	var RES []byte

	for i := 0; i < len(dataArray); i += attributeLength {
		attributeLength = int(uint(dataArray[1+i])) * 4
		attributeType = int(uint(dataArray[0+i]))

		if attributeType == ausf_context.AT_RES_ATTRIBUTE {
			logger.EapAuthComfirmLog.Infoln("Detect AT_RES attribute")
			detectRes = true
			resLength := int(uint(dataArray[3+i]) | uint(dataArray[2+i])<<8)
			RES = dataArray[4+i : 4+i+attributeLength-4]
			byteRes := padZeros(RES, resLength)
			RES = byteRes
		} else if attributeType == ausf_context.AT_MAC_ATTRIBUTE {
			logger.EapAuthComfirmLog.Infoln("Detect AT_MAC attribute")
			detectMac = true
			macStr := string(dataArray[4+i : 20+i])
			if checkMACintegrity(i, []byte(macStr), wholePacket, Kautn) {
				logger.EapAuthComfirmLog.Infoln("check MAC integrity succeed")
				macCorrect = true
			} else {
				logger.EapAuthComfirmLog.Infoln("check MAC integrity failed")
			}
		} else {
			logger.EapAuthComfirmLog.Infof("Detect unknown attribute with type %d\n", attributeType)
		}
	}
	if detectRes && detectMac && macCorrect {
		return RES, true
	}
	return nil, false
}

func ConstructFailEapAkaNotification(oldPktId uint8) string {
	var eapPkt radius.EapPacket
	eapPkt.Code = radius.EapCodeRequest
	eapPkt.Identifier = oldPktId + 1
	eapPkt.Type = ausf_context.EAP_AKA_PRIME_TYPENUM
	attrNum := fmt.Sprintf("%02x", ausf_context.AT_NOTIFICATION_ATTRIBUTE)
	attribute := attrNum + "01" + "4000"
	var attrHex []byte
	if attrHexTmp, err := hex.DecodeString(attribute); err != nil {
		logger.EapAuthComfirmLog.Warnf("Decode attribute failed: %+v", err)
	} else {
		attrHex = attrHexTmp
	}
	eapPkt.Data = attrHex
	eapPktEncode := eapPkt.Encode()
	return base64.StdEncoding.EncodeToString(eapPktEncode)
}

func ConstructEapNoTypePkt(code radius.EapCode, pktID uint8) string {
	b := make([]byte, 4)
	b[0] = byte(code)
	b[1] = pktID
	binary.BigEndian.PutUint16(b[2:4], uint16(4))
	return base64.StdEncoding.EncodeToString(b)
}

func getUdmUrl(nrfUri string) string {
	udmUrl := "https://localhost:29503" // default
	nfDiscoverParam := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NUDM_UEAU}),
	}
	res, err := consumer.SendSearchNFInstances(nrfUri, models.NfType_UDM, models.NfType_AUSF, nfDiscoverParam)
	if err != nil {
		logger.UeAuthPostLog.Errorln("[Search UDM UEAU] ", err.Error())
	} else if len(res.NfInstances) > 0 {
		udmInstance := res.NfInstances[0]
		if len(udmInstance.Ipv4Addresses) > 0 && udmInstance.NfServices != nil {
			ueauService := (*udmInstance.NfServices)[0]
			ueauEndPoint := (*ueauService.IpEndPoints)[0]
			udmUrl = string(ueauService.Scheme) + "://" + ueauEndPoint.Ipv4Address + ":" + strconv.Itoa(int(ueauEndPoint.Port))
		}
	} else {
		logger.UeAuthPostLog.Errorln("[Search UDM UEAU] len(NfInstances) = 0")
	}
	return udmUrl
}

func createClientToUdmUeau(udmUrl string) *Nudm_UEAU.APIClient {
	cfg := Nudm_UEAU.NewConfiguration()
	cfg.SetBasePath(udmUrl)
	clientAPI := Nudm_UEAU.NewAPIClient(cfg)
	return clientAPI
}

func sendAuthResultToUDM(id string, authType models.AuthType, success bool, servingNetworkName, udmUrl string) error {
	timeNow := time.Now()
	timePtr := &timeNow

	var authEvent models.AuthEvent
	authEvent.TimeStamp = timePtr
	authEvent.AuthType = authType
	authEvent.Success = success
	authEvent.ServingNetworkName = servingNetworkName

	client := createClientToUdmUeau(udmUrl)
	_, _, confirmAuthErr := client.ConfirmAuthApi.ConfirmAuth(context.Background(), id, authEvent)
	return confirmAuthErr
}

func logConfirmFailureAndInformUDM(id string, authType models.AuthType, servingNetworkName, errStr, udmUrl string) {
	if authType == models.AuthType__5_G_AKA {
		logger.Auth5gAkaComfirmLog.Infoln(errStr)
		if sendErr := sendAuthResultToUDM(id, authType, false, "", udmUrl); sendErr != nil {
			logger.Auth5gAkaComfirmLog.Infoln(sendErr.Error())
		}
	} else if authType == models.AuthType_EAP_AKA_PRIME {
		logger.EapAuthComfirmLog.Infoln(errStr)
		if sendErr := sendAuthResultToUDM(id, authType, false, "", udmUrl); sendErr != nil {
			logger.EapAuthComfirmLog.Infoln(sendErr.Error())
		}
	}
}
