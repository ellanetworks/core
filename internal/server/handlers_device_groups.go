package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	db "github.com/yeastengine/ella/internal/db/sql"
)

type CreateDeviceGroupParams struct {
	Name             string `json:"name"`
	SiteInfo         string `json:"site_info"`
	IpDomainName     string `json:"ip_domain_name"`
	Dnn              string `json:"dnn"`
	UeIpPool         string `json:"ue_ip_pool"`
	DnsPrimary       string `json:"dns_primary"`
	Mtu              int64  `json:"mtu"`
	DnnMbrUplink     int64  `json:"dnn_mbr_uplink"`
	DnnMbrDownlink   int64  `json:"dnn_mbr_downlink"`
	TrafficClassName string `json:"traffic_class_name"`
	TrafficClassArp  int64  `json:"traffic_class_arp"`
	TrafficClassPdb  int64  `json:"traffic_class_pdb"`
	TrafficClassPelr int64  `json:"traffic_class_pelr"`
	TrafficClassQci  int64  `json:"traffic_class_qci"`
	NetworkSliceId   int64  `json:"network_slice_id"`
}

type CreateDeviceGroupResponse struct {
	ID int64 `json:"id"`
}

type GetDeviceGroupResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	SiteInfo         string `json:"site_info"`
	IpDomainName     string `json:"ip_domain_name"`
	Dnn              string `json:"dnn"`
	UeIpPool         string `json:"ue_ip_pool"`
	DnsPrimary       string `json:"dns_primary"`
	Mtu              int64  `json:"mtu"`
	DnnMbrUplink     int64  `json:"dnn_mbr_uplink"`
	DnnMbrDownlink   int64  `json:"dnn_mbr_downlink"`
	TrafficClassName string `json:"traffic_class_name"`
	TrafficClassArp  int64  `json:"traffic_class_arp"`
	TrafficClassPdb  int64  `json:"traffic_class_pdb"`
	TrafficClassPelr int64  `json:"traffic_class_pelr"`
	TrafficClassQci  int64  `json:"traffic_class_qci"`
	NetworkSliceId   int64  `json:"network_slice_id"`
}

type DeleteDeviceGroupResponse struct {
	ID int64 `json:"id"`
}

func ListDeviceGroups(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceGroups, err := env.DBQueries.ListDeviceGroups(context.Background())
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(deviceGroups))
		for _, deviceGroup := range deviceGroups {
			ids = append(ids, deviceGroup.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateDeviceGroup(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var deviceGroup CreateDeviceGroupParams
		if err := json.NewDecoder(r.Body).Decode(&deviceGroup); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if deviceGroup.Name == "" {
			writeError(w, http.StatusBadRequest, "`name` is required")
			return
		}
		if deviceGroup.SiteInfo == "" {
			writeError(w, http.StatusBadRequest, "`site_info` is required")
			return
		}
		if deviceGroup.IpDomainName == "" {
			writeError(w, http.StatusBadRequest, "`ip_domain_name` is required")
			return
		}
		if deviceGroup.Dnn == "" {
			writeError(w, http.StatusBadRequest, "`dnn` is required")
			return
		}
		if deviceGroup.UeIpPool == "" {
			writeError(w, http.StatusBadRequest, "`ue_ip_pool` is required")
			return
		}
		if deviceGroup.DnsPrimary == "" {
			writeError(w, http.StatusBadRequest, "`dns_primary` is required")
			return
		}
		if deviceGroup.Mtu == 0 {
			writeError(w, http.StatusBadRequest, "`mtu` is required")
			return
		}
		if deviceGroup.DnnMbrUplink == 0 {
			writeError(w, http.StatusBadRequest, "`dnn_mbr_uplink` is required")
			return
		}
		if deviceGroup.DnnMbrDownlink == 0 {
			writeError(w, http.StatusBadRequest, "`dnn_mbr_downlink` is required")
			return
		}
		if deviceGroup.TrafficClassName == "" {
			writeError(w, http.StatusBadRequest, "`traffic_class_name` is required")
			return
		}
		if deviceGroup.TrafficClassArp == 0 {
			writeError(w, http.StatusBadRequest, "`traffic_class_arp` is required")
			return
		}
		if deviceGroup.TrafficClassPdb == 0 {
			writeError(w, http.StatusBadRequest, "`traffic_class_pdb` is required")
			return
		}
		if deviceGroup.TrafficClassPelr == 0 {
			writeError(w, http.StatusBadRequest, "`traffic_class_pelr` is required")
			return
		}
		if deviceGroup.TrafficClassQci == 0 {
			writeError(w, http.StatusBadRequest, "`traffic_class_qci` is required")
			return
		}
		if deviceGroup.NetworkSliceId <= 0 {
			writeError(w, http.StatusBadRequest, "`network_slice_id` is required")
			return
		}

		var networkSliceId sql.NullInt64
		if deviceGroup.NetworkSliceId != 0 {
			_, err := env.DBQueries.GetNetworkSlice(context.Background(), deviceGroup.NetworkSliceId)
			if err != nil {
				if err == sql.ErrNoRows {
					writeError(w, http.StatusBadRequest, "network slice not found")
					return
				}
				log.Println(err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			networkSliceId = sql.NullInt64{
				Int64: deviceGroup.NetworkSliceId,
			}
		}

		var poolID int64
		existingPool, err := env.DBQueries.GetIPPoolByCIDR(context.Background(), deviceGroup.UeIpPool)
		if err == sql.ErrNoRows {
			newPool, err := env.DBQueries.CreateIPPool(context.Background(), deviceGroup.UeIpPool)
			if err != nil {
				log.Println("failed to create IP pool:", err)
				writeError(w, http.StatusInternalServerError, "internal error creating IP pool")
				return
			}
			poolID = newPool
		} else if err != nil {
			log.Println("failed to check existing IP pool:", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else {
			poolID = existingPool.ID
		}

		dbDeviceGroup := db.CreateDeviceGroupParams{
			Name:             deviceGroup.Name,
			SiteInfo:         deviceGroup.SiteInfo,
			IpDomainName:     deviceGroup.IpDomainName,
			Dnn:              deviceGroup.Dnn,
			UeIpPoolID:       poolID,
			DnsPrimary:       deviceGroup.DnsPrimary,
			Mtu:              deviceGroup.Mtu,
			DnnMbrUplink:     deviceGroup.DnnMbrUplink,
			DnnMbrDownlink:   deviceGroup.DnnMbrDownlink,
			TrafficClassName: deviceGroup.TrafficClassName,
			TrafficClassArp:  deviceGroup.TrafficClassArp,
			TrafficClassPdb:  deviceGroup.TrafficClassPdb,
			TrafficClassPelr: deviceGroup.TrafficClassPelr,
			TrafficClassQci:  deviceGroup.TrafficClassQci,
			NetworkSliceID:   networkSliceId.Int64,
		}
		newDeviceGroup, err := env.DBQueries.CreateDeviceGroup(context.Background(), dbDeviceGroup)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateDeviceGroupResponse{ID: newDeviceGroup.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetDeviceGroup(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		deviceGroup, err := env.DBQueries.GetDeviceGroup(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "DeviceGroup not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		ueIpPool, err := env.DBQueries.GetIPPoolCIDR(context.Background(), deviceGroup.UeIpPoolID)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		deviceGroupResponse := GetDeviceGroupResponse{
			ID:               deviceGroup.ID,
			Name:             deviceGroup.Name,
			SiteInfo:         deviceGroup.SiteInfo,
			IpDomainName:     deviceGroup.IpDomainName,
			Dnn:              deviceGroup.Dnn,
			UeIpPool:         ueIpPool,
			DnsPrimary:       deviceGroup.DnsPrimary,
			Mtu:              deviceGroup.Mtu,
			DnnMbrUplink:     deviceGroup.DnnMbrUplink,
			DnnMbrDownlink:   deviceGroup.DnnMbrDownlink,
			TrafficClassName: deviceGroup.TrafficClassName,
			TrafficClassArp:  deviceGroup.TrafficClassArp,
			TrafficClassPdb:  deviceGroup.TrafficClassPdb,
			TrafficClassPelr: deviceGroup.TrafficClassPelr,
			TrafficClassQci:  deviceGroup.TrafficClassQci,
			NetworkSliceId:   deviceGroup.NetworkSliceID,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, deviceGroupResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteDeviceGroup(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		err = env.DBQueries.DeleteDeviceGroup(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteDeviceGroupResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
