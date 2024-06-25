package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type NodeAssociationNoSession struct {
	ID            string
	Addr          string
	NextSessionID uint64
}

type NodeAssociationMapNoSession map[string]NodeAssociationNoSession

func (h *ApiHandler) listPfcpAssociations(c *gin.Context) {
	nodeAssociationsNoSession := make(NodeAssociationMapNoSession)
	for k, v := range h.PfcpSrv.NodeAssociations {
		nodeAssociationsNoSession[k] = NodeAssociationNoSession{
			ID:            v.ID,
			Addr:          v.Addr,
			NextSessionID: v.NextSessionID,
		}
	}
	c.IndentedJSON(http.StatusOK, nodeAssociationsNoSession)
}

func (h *ApiHandler) listPfcpAssociationsFull(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, h.PfcpSrv.NodeAssociations)
}
