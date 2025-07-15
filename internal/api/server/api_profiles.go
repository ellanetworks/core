package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateProfileParams struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type GetProfileResponse struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

const (
	ListProfilesAction  = "list_profiles"
	GetProfileAction    = "get_profile"
	CreateProfileAction = "create_profile"
	UpdateProfileAction = "update_profile"
	DeleteProfileAction = "delete_profile"
)

func isProfileNameValid(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func isUeIPPoolValid(ueIPPool string) bool {
	_, _, err := net.ParseCIDR(ueIPPool)
	return err == nil
}

func isValidDNS(dns string) bool {
	return net.ParseIP(dns) != nil
}

func isValidMTU(mtu int32) bool {
	return mtu >= 0 && mtu <= 65535
}

func isValidBitrate(bitrate string) bool {
	s := strings.Split(bitrate, " ")
	if len(s) != 2 {
		return false
	}
	value := s[0]
	unit := s[1]
	if unit != "Mbps" && unit != "Gbps" {
		return false
	}

	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	return valueInt > 0 && valueInt <= 1000
}

func isValid5Qi(var5qi int32) bool {
	return var5qi >= 1 && var5qi <= 255
}

func isValidPriorityLevel(priorityLevel int32) bool {
	return priorityLevel >= 1 && priorityLevel <= 255
}

func ListProfiles(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		dbProfiles, err := dbInstance.ListProfiles(r.Context())
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Profiles not found", err, logger.APILog)
			return
		}
		profileList := make([]GetProfileResponse, 0)
		for _, dbProfile := range dbProfiles {
			profileList = append(profileList, GetProfileResponse{
				Name:            dbProfile.Name,
				UeIPPool:        dbProfile.UeIPPool,
				DNS:             dbProfile.DNS,
				Mtu:             dbProfile.Mtu,
				BitrateDownlink: dbProfile.BitrateDownlink,
				BitrateUplink:   dbProfile.BitrateUplink,
				Var5qi:          dbProfile.Var5qi,
				PriorityLevel:   dbProfile.PriorityLevel,
			})
		}
		writeResponse(w, profileList, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(ListProfilesAction, email, getClientIP(r), "User listed profiles")
	})
}

func GetProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/profiles/")
		if name == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		dbProfile, err := dbInstance.GetProfile(r.Context(), name)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Profile not found", err, logger.APILog)
			return
		}
		profile := GetProfileResponse{
			Name:            dbProfile.Name,
			UeIPPool:        dbProfile.UeIPPool,
			DNS:             dbProfile.DNS,
			Mtu:             dbProfile.Mtu,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
		writeResponse(w, profile, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(GetProfileAction, email, getClientIP(r), "User retrieved profile: "+name)
	})
}

func DeleteProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/profiles/")
		if name == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		_, err := dbInstance.GetProfile(r.Context(), name)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Profile not found", err, logger.APILog)
			return
		}
		subsInProfile, err := dbInstance.SubscribersInProfile(r.Context(), name)
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to check subscribers", err, logger.APILog)
			return
		}
		if subsInProfile {
			writeErrorHTTP(w, http.StatusConflict, "Profile has subscribers", nil, logger.APILog)
			return
		}
		if err := dbInstance.DeleteProfile(r.Context(), name); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to delete profile", err, logger.APILog)
			return
		}
		writeResponse(w, SuccessResponse{Message: "Profile deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteProfileAction, email, getClientIP(r), "User deleted profile: "+name)
	})
}

func CreateProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var createProfileParams CreateProfileParams
		if err := json.NewDecoder(r.Body).Decode(&createProfileParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateProfileParams(createProfileParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetProfile(r.Context(), createProfileParams.Name); err == nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Profile already exists", nil, logger.APILog)
			return
		}

		dbProfile := &db.Profile{
			Name:            createProfileParams.Name,
			UeIPPool:        createProfileParams.UeIPPool,
			DNS:             createProfileParams.DNS,
			Mtu:             createProfileParams.Mtu,
			BitrateDownlink: createProfileParams.BitrateDownlink,
			BitrateUplink:   createProfileParams.BitrateUplink,
			Var5qi:          createProfileParams.Var5qi,
			PriorityLevel:   createProfileParams.PriorityLevel,
		}

		if err := dbInstance.CreateProfile(r.Context(), dbProfile); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to create profile", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Profile created successfully"}, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreateProfileAction, email, getClientIP(r), "User created profile: "+createProfileParams.Name)
	})
}

func UpdateProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		groupName := strings.TrimPrefix(r.URL.Path, "/api/v1/profiles/")
		if groupName == "" || strings.ContainsRune(groupName, '/') {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid or missing name parameter", nil, logger.APILog)
			return
		}

		var updateProfileParams CreateProfileParams
		if err := json.NewDecoder(r.Body).Decode(&updateProfileParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateProfileParams(updateProfileParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		profile, err := dbInstance.GetProfile(r.Context(), groupName)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Profile not found", err, logger.APILog)
			return
		}

		profile.Name = updateProfileParams.Name
		profile.UeIPPool = updateProfileParams.UeIPPool
		profile.DNS = updateProfileParams.DNS
		profile.Mtu = updateProfileParams.Mtu
		profile.BitrateDownlink = updateProfileParams.BitrateDownlink
		profile.BitrateUplink = updateProfileParams.BitrateUplink
		profile.Var5qi = updateProfileParams.Var5qi
		profile.PriorityLevel = updateProfileParams.PriorityLevel

		if err := dbInstance.UpdateProfile(r.Context(), profile); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to update profile", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Profile updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateProfileAction, email, getClientIP(r), "User updated profile: "+updateProfileParams.Name)
	})
}

func validateProfileParams(p CreateProfileParams) error {
	switch {
	case p.Name == "":
		return errors.New("name is missing")
	case p.UeIPPool == "":
		return errors.New("ue-ip-pool is missing")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.Mtu == 0:
		return errors.New("mtu is missing")
	case p.BitrateUplink == "":
		return errors.New("bitrate-uplink is missing")
	case p.BitrateDownlink == "":
		return errors.New("bitrate-downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.PriorityLevel == 0:
		return errors.New("priority-level is missing")
	case !isProfileNameValid(p.Name):
		return errors.New("Invalid name format. Must be less than 256 characters")
	case !isUeIPPoolValid(p.UeIPPool):
		return errors.New("Invalid ue-ip-pool format. Must be in CIDR format")
	case !isValidDNS(p.DNS):
		return errors.New("Invalid dns format. Must be a valid IP address")
	case !isValidMTU(p.Mtu):
		return errors.New("Invalid mtu format. Must be an integer between 0 and 65535")
	case !isValidBitrate(p.BitrateUplink):
		return errors.New("Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValidBitrate(p.BitrateDownlink):
		return errors.New("Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("Invalid Var5qi format. Must be an integer between 1 and 255")
	case !isValidPriorityLevel(p.PriorityLevel):
		return errors.New("Invalid priority-level format. Must be an integer between 1 and 255")
	}
	return nil
}
