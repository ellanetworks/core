package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"upf-adapter/config"
	"upf-adapter/logger"
	"upf-adapter/pfcp"
	"upf-adapter/pfcp/udp"
)

// Hnadler for SMF initiated msgs
func handler(w http.ResponseWriter, req *http.Request) {
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.AppLog.Errorf("server: could not read request body: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	var udpPodMsg config.UdpPodPfcpMsg
	json.Unmarshal(reqBody, &udpPodMsg)

	logger.AppLog.Debugf("\n received msg type [%v], upf nodeId [%s], smfIp [%v], msg [%v]",
		udpPodMsg.Msg.Header.MessageType, udpPodMsg.UpNodeID.NodeIdValue, udpPodMsg.SmfIp, udpPodMsg.Msg)

	pfcpJsonRsp, err := pfcp.ForwardPfcpMsgToUpf(udpPodMsg)
	if err != nil {
		logger.AppLog.Errorf("Error HttpLib received pfcp Rsp ")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(pfcpJsonRsp)
	logger.AppLog.Debugf("response sent for %v", udpPodMsg.Msg.Header.MessageType)
}

// UDP handler for pfcp msg from UPF
func init() {
	go udp.Run(pfcp.Dispatch)
}

// Handler for msgs from SMF
func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8090", nil)
}
