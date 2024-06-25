package rest

import (
	"net/http"

	"github.com/yeastengine/ella/internal/upf/config"
	"github.com/yeastengine/ella/internal/upf/core"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/upf/ebpf"
)

type ApiHandler struct {
	BpfObjects        *ebpf.BpfObjects
	PfcpSrv           *core.PfcpConnection
	ForwardPlaneStats *ebpf.UpfXdpActionStatistic
	Cfg               *config.UpfConfig
}

func NewApiHandler(bpfObjects *ebpf.BpfObjects, pfcpSrv *core.PfcpConnection, forwardPlaneStats *ebpf.UpfXdpActionStatistic, cfg *config.UpfConfig) *ApiHandler {
	return &ApiHandler{
		BpfObjects:        bpfObjects,
		PfcpSrv:           pfcpSrv,
		ForwardPlaneStats: forwardPlaneStats,
		Cfg:               cfg,
	}
}

func (h *ApiHandler) InitRoutes() *gin.Engine {
	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	router.Use(cors.New(config))

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			c.IndentedJSON(http.StatusOK, "OK")
		})

		h.initDefaultRoutes(v1)
	}

	return router
}

func (h *ApiHandler) initDefaultRoutes(group *gin.RouterGroup) {

	group.GET("xdp_stats", h.displayXdpStatistics)
	group.GET("packet_stats", h.displayPacketStats)

	config := group.Group("config")
	{
		config.GET("", h.displayConfig)
		config.POST("", h.editConfig)
	}

	pdrMap := group.Group("uplink_pdr_map")
	{
		pdrMap.GET(":id", h.getUplinkPdrValue)
		pdrMap.PUT(":id", h.setUplinkPdrValue)
	}

	qerMap := group.Group("qer_map")
	{
		qerMap.GET("", h.listQerMapContent)
		qerMap.GET(":id", h.getQerValue)
		qerMap.PUT(":id", h.setQerValue)
	}

	farMap := group.Group("far_map")
	{
		farMap.GET(":id", h.getFarValue)
		farMap.PUT(":id", h.setFarValue)
	}

	associations := group.Group("pfcp_associations")
	{
		associations.GET("", h.listPfcpAssociations)
		associations.GET("full", h.listPfcpAssociationsFull)
	}

	sessions := group.Group("pfcp_sessions")
	{
		//sessions.GET("", ListPfcpSessions(pfcpSrv))
		sessions.GET("", h.listPfcpSessionsFiltered)
	}
}
