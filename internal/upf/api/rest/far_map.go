package rest

import (
	"net/http"
	"strconv"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/upf/ebpf"
)

type FarMapElement struct {
	Id                    uint32 `json:"id"`
	Action                uint8  `json:"action"`
	OuterHeaderCreation   uint8  `json:"outer_header_creation"`
	Teid                  uint32 `json:"teid"`
	RemoteIP              uint32 `json:"remote_ip"`
	LocalIP               uint32 `json:"local_ip"`
	TransportLevelMarking uint16 `json:"transport_level_marking"`
}

func (h *ApiHandler) getFarValue(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		logger.AppLog.Printf("Not an integer id: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var value ebpf.FarInfo
	if err = h.BpfObjects.IpEntrypointObjects.FarMap.Lookup(uint32(id), unsafe.Pointer(&value)); err != nil {
		logger.AppLog.Printf("Error reading map: %s", err.Error())
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, FarMapElement{
		Id:                    uint32(id),
		Action:                value.Action,
		OuterHeaderCreation:   value.OuterHeaderCreation,
		Teid:                  value.Teid,
		RemoteIP:              value.RemoteIP,
		LocalIP:               value.LocalIP,
		TransportLevelMarking: value.TransportLevelMarking,
	})
}

func (h *ApiHandler) setFarValue(c *gin.Context) {
	var farElement FarMapElement
	if err := c.BindJSON(&farElement); err != nil {
		logger.AppLog.Printf("Parsing request body error: %s", err.Error())
		return
	}

	value := ebpf.FarInfo{
		Action:                farElement.Action,
		OuterHeaderCreation:   farElement.OuterHeaderCreation,
		Teid:                  farElement.Teid,
		RemoteIP:              farElement.RemoteIP,
		LocalIP:               farElement.LocalIP,
		TransportLevelMarking: farElement.TransportLevelMarking,
	}

	if err := h.BpfObjects.IpEntrypointObjects.FarMap.Put(farElement.Id, unsafe.Pointer(&value)); err != nil {
		logger.AppLog.Printf("Error writing map: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, farElement)
}
