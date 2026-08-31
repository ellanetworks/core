// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/client"
)

var errRequester = errors.New("requester failure")

func TestClientMethodsPropagateRequesterErrors(t *testing.T) {
	tests := []struct {
		name string
		call func(*testing.T, context.Context, *client.Client) error
	}{
		{"CreateBackup", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createBackupOpts := &client.CreateBackupParams{
				Path: "invalid/path",
			}

			return c.CreateBackup(ctx, createBackupOpts)
		}},
		{"RestoreBackup", func(t *testing.T, ctx context.Context, c *client.Client) error {
			path := filepath.Join(t.TempDir(), "ella_core.backup")
			if err := os.WriteFile(path, []byte("backup data"), 0o600); err != nil {
				t.Fatalf("write temp backup file: %v", err)
			}

			return c.RestoreBackup(ctx, &client.RestoreBackupParams{Path: path})
		}},
		{"GetBGPSettings", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetBGPSettings(ctx)

			return err
		}},
		{"UpdateBGPSettings", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateBGPSettingsOptions{
				LocalAS: 0,
			}

			return c.UpdateBGPSettings(ctx, opts)
		}},
		{"ListBGPPeers", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{Page: 1, PerPage: 25}

			_, err := c.ListBGPPeers(ctx, params)

			return err
		}},
		{"GetBGPPeer", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: 999})

			return err
		}},
		{"CreateBGPPeer", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.CreateBGPPeerOptions{
				Address:  "10.0.0.2",
				RemoteAS: 65001,
			}

			return c.CreateBGPPeer(ctx, opts)
		}},
		{"UpdateBGPPeer", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateBGPPeerOptions{ID: 7}

			return c.UpdateBGPPeer(ctx, opts)
		}},
		{"DeleteBGPPeer", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: 999})
		}},
		{"GetBGPAdvertisedRoutes", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetBGPAdvertisedRoutes(ctx)

			return err
		}},
		{"GetBGPLearnedRoutes", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetBGPLearnedRoutes(ctx)

			return err
		}},
		{"GetCellPosition", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetCellPosition(ctx, "missing")

			return err
		}},
		{"CreateCellPosition", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.CreateCellPosition(ctx, &client.CreateCellPositionOptions{RAT: "nr"})
		}},
		{"DeleteCellPosition", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteCellPosition(ctx, "missing")
		}},
		{"ListClusterMembers", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.ListClusterMembers(ctx)

			return err
		}},
		{"DrainClusterMember", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.DrainClusterMember(ctx, 1)

			return err
		}},
		{"PromoteClusterMember", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.PromoteClusterMember(ctx, 99)
		}},
		{"RemoveClusterMember", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.RemoveClusterMember(ctx, 2, false)
		}},
		{"CreateDataNetwork", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createDataNetworkOpts := &client.CreateDataNetworkOptions{
				Name:     "testDataNetwork",
				IPv4Pool: "12312312312",
				DNS:      "8.8.8.8",
				Mtu:      1400,
			}

			return c.CreateDataNetwork(ctx, createDataNetworkOpts)
		}},
		{"GetDataNetwork", func(t *testing.T, ctx context.Context, c *client.Client) error {
			name := "non-existent-data-network"
			getDataNetworkOpts := &client.GetDataNetworkOptions{
				Name: name,
			}

			_, err := c.GetDataNetwork(ctx, getDataNetworkOpts)

			return err
		}},
		{"DeleteDataNetwork", func(t *testing.T, ctx context.Context, c *client.Client) error {
			name := "non-existent-data-network"

			deleteDataNetworkOpts := &client.DeleteDataNetworkOptions{
				Name: name,
			}

			return c.DeleteDataNetwork(ctx, deleteDataNetworkOpts)
		}},
		{"ListDataNetworks", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListDataNetworks(ctx, params)

			return err
		}},
		{"ListIPv4Allocations", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.ListIPAllocationsOptions{
				DataNetworkName: "nonexistent",
			}

			params := &client.ListParams{
				Page:    1,
				PerPage: 25,
			}

			_, err := c.ListIPv4Allocations(ctx, opts, params)

			return err
		}},
		{"ListIPv6Allocations", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.ListIPAllocationsOptions{
				DataNetworkName: "nonexistent",
			}

			params := &client.ListParams{
				Page:    1,
				PerPage: 25,
			}

			_, err := c.ListIPv6Allocations(ctx, opts, params)

			return err
		}},
		{"ListDataNetworkStaticIps", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.ListDataNetworkStaticIps(ctx, "nonexistent")

			return err
		}},
		{"CreateDataNetworkStaticIp", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.CreateDataNetworkStaticIp(ctx, "internet", &client.CreateStaticIPOptions{
				IMSI:    "001010000000001",
				Address: "10.45.0.10",
			})
		}},
		{"UpdateDataNetworkStaticIp", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.UpdateDataNetworkStaticIp(ctx, "internet", "001010000000001", "ipv4", "10.45.0.20")
		}},
		{"DeleteDataNetworkStaticIp", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteDataNetworkStaticIp(ctx, "internet", "001010000000001", "ipv4")
		}},
		{"GetFlowAccountingInfo", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetFlowAccountingInfo(ctx)

			return err
		}},
		{"UpdateFlowAccountingInfo", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateFlowAccountingInfoOptions{
				Enabled: false,
			}

			return c.UpdateFlowAccountingInfo(ctx, opts)
		}},
		{"ListFlowReports", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListFlowReportsParams{
				Page:    1,
				PerPage: 25,
			}

			_, err := c.ListFlowReports(ctx, params)

			return err
		}},
		{"ListFlowReportsByDay", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListFlowReportsParams{
				Start: "2026-02-20",
				End:   "2026-02-21",
			}

			_, err := c.ListFlowReportsByDay(ctx, params)

			return err
		}},
		{"ListFlowReportsBySubscriber", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListFlowReportsParams{
				Start: "2026-02-20",
				End:   "2026-02-21",
			}

			_, err := c.ListFlowReportsBySubscriber(ctx, params)

			return err
		}},
		{"ClearFlowReports", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.ClearFlowReports(ctx)
		}},
		{"GetFlowReportsRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetFlowReportsRetentionPolicy(ctx)

			return err
		}},
		{"UpdateFlowReportsRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOpts := &client.UpdateFlowReportsRetentionPolicyOptions{
				Days: -10,
			}

			return c.UpdateFlowReportsRetentionPolicy(ctx, updateOpts)
		}},
		{"GetFlowReportStats", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListFlowReportsParams{}

			_, err := c.GetFlowReportStats(ctx, params)

			return err
		}},
		{"Initialize", func(t *testing.T, ctx context.Context, c *client.Client) error {
			initializeOpts := &client.InitializeOptions{
				Email:    "invalid-email",
				Password: "secret",
			}

			return c.Initialize(ctx, initializeOpts)
		}},
		{"ListNetworkInterfaces", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.ListNetworkInterfaces(ctx)

			return err
		}},
		{"UpdateN3Interface", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateN3InterfaceOptions{
				ExternalAddress: "not-an-ip",
			}

			return c.UpdateN3Interface(ctx, opts)
		}},
		{"Login", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.Login(ctx, &client.LoginOptions{Email: "user@example.com", Password: "secret"})
		}},
		{"Refresh", func(t *testing.T, ctx context.Context, c *client.Client) error {
			c.SetToken("oldtoken")

			return c.Refresh(ctx)
		}},
		{"ListAuditLogs", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListAuditLogsParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListAuditLogs(ctx, params)

			return err
		}},
		{"GetAuditLogRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetAuditLogRetentionPolicy(ctx)

			return err
		}},
		{"UpdateAuditLogRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOpts := &client.UpdateAuditLogsRetentionPolicyOptions{
				Days: -10,
			}

			return c.UpdateAuditLogRetentionPolicy(ctx, updateOpts)
		}},
		{"ListAuditLogsByActor", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListAuditLogsByActor(ctx, "admin@example.com", params)

			return err
		}},
		{"GetNATInfo", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetNATInfo(ctx)

			return err
		}},
		{"UpdateNATInfo", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateNATInfoOpts := &client.UpdateNATInfoOptions{
				Enabled: false,
			}

			return c.UpdateNATInfo(ctx, updateNATInfoOpts)
		}},
		{"GetOperator", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetOperator(ctx)

			return err
		}},
		{"UpdateOperatorID", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOperatorIDOpts := &client.UpdateOperatorIDOptions{
				Mcc: "001",
				Mnc: "01",
			}

			return c.UpdateOperatorID(ctx, updateOperatorIDOpts)
		}},
		{"UpdateOperatorTracking", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOperatorTrackingOpts := &client.UpdateOperatorTrackingOptions{
				SupportedTacs: []string{"001", "002"},
			}

			return c.UpdateOperatorTracking(ctx, updateOperatorTrackingOpts)
		}},
		{"UpdateOperatorNASSecurity", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOperatorNASSecurityOpts := &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"INVALID"},
				Integrity: []string{"AES"},
			}

			return c.UpdateOperatorNASSecurity(ctx, updateOperatorNASSecurityOpts)
		}},
		{"DeleteHomeNetworkKey", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteHomeNetworkKey(ctx, "0190b3d2-7c12-7c00-8000-000000000999")
		}},
		{"GetHomeNetworkKeyPrivateKey", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetHomeNetworkKeyPrivateKey(ctx, "0190b3d2-7c12-7c00-8000-000000000999")

			return err
		}},
		{"UpdateOperatorSPN", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateOperatorSPNOptions{
				FullName:  "",
				ShortName: "Ella",
			}

			return c.UpdateOperatorSPN(ctx, opts)
		}},
		{"UpdateOperatorCode", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{
				OperatorCode: "not-hex",
			})
		}},
		{"CreatePolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createPolicyOpts := &client.CreatePolicyOptions{
				Name:                "testPolicy",
				ProfileName:         "testProfile",
				SliceName:           "default",
				DataNetworkName:     "internet",
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 1,
			}

			return c.CreatePolicy(ctx, createPolicyOpts)
		}},
		{"GetPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			name := "non-existent-policy"
			getPolicyOpts := &client.GetPolicyOptions{
				Name: name,
			}

			_, err := c.GetPolicy(ctx, getPolicyOpts)

			return err
		}},
		{"DeletePolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			name := "non-existent-policy"

			deletePolicyOpts := &client.DeletePolicyOptions{
				Name: name,
			}

			return c.DeletePolicy(ctx, deletePolicyOpts)
		}},
		{"ListPolicies", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListPolicies(ctx, params)

			return err
		}},
		{"UpdatePolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updatePolicyOpts := &client.UpdatePolicyOptions{
				SessionAmbrUplink: "150 Mbps",
			}

			return c.UpdatePolicy(ctx, "non-existent-policy", updatePolicyOpts)
		}},
		{"ListPositioningSessions", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.ListPositioningSessions(ctx, "imsi-001010000000001")

			return err
		}},
		{"CreateProfile", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.CreateProfileOptions{
				Name:           "enterprise",
				UeAmbrUplink:   "1 Gbps",
				UeAmbrDownlink: "1 Gbps",
			}

			return c.CreateProfile(ctx, opts)
		}},
		{"GetProfile", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetProfile(ctx, &client.GetProfileOptions{Name: "non-existent"})

			return err
		}},
		{"UpdateProfile", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateProfileOptions{
				UeAmbrUplink: "2 Gbps",
			}

			return c.UpdateProfile(ctx, "non-existent", opts)
		}},
		{"DeleteProfile", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: "enterprise"})
		}},
		{"ListProfiles", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListProfiles(ctx, params)

			return err
		}},
		{"GetRadio", func(t *testing.T, ctx context.Context, c *client.Client) error {
			getRadioOpts := &client.GetRadioOptions{
				RanNodeType: "gNB",
				ID:          "ffffff",
			}

			_, err := c.GetRadio(ctx, getRadioOpts)

			return err
		}},
		{"ListRadios", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListRadios(ctx, params)

			return err
		}},
		{"ListRadioEvents", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListRadioEventsParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListRadioEvents(ctx, params)

			return err
		}},
		{"GetRadioEventRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetRadioEventRetentionPolicy(ctx)

			return err
		}},
		{"UpdateRadioEventRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOpts := &client.UpdateRadioEventsRetentionPolicyOptions{
				Days: 0,
			}

			return c.UpdateRadioEventRetentionPolicy(ctx, updateOpts)
		}},
		{"GetRadioEvent", func(t *testing.T, ctx context.Context, c *client.Client) error {
			logID := 999

			_, err := c.GetRadioEvent(ctx, logID)

			return err
		}},
		{"ForgetRadio", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.ForgetRadio(ctx, &client.ForgetRadioOptions{RanNodeType: "gNB", ID: "000102"})
		}},
		{"CreateRoute", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createRouteOpts := &client.CreateRouteOptions{
				Destination: "invalid_destination",
				Gateway:     "1.2.3.1",
				Interface:   "eth0",
				Metric:      100,
			}

			return c.CreateRoute(ctx, createRouteOpts)
		}},
		{"GetRoute", func(t *testing.T, ctx context.Context, c *client.Client) error {
			var id int64 = 123

			getRouteOpts := &client.GetRouteOptions{
				ID: id,
			}

			_, err := c.GetRoute(ctx, getRouteOpts)

			return err
		}},
		{"DeleteRoute", func(t *testing.T, ctx context.Context, c *client.Client) error {
			var id int64 = 123

			deleteRouteOpts := &client.DeleteRouteOptions{
				ID: id,
			}

			return c.DeleteRoute(ctx, deleteRouteOpts)
		}},
		{"ListRoutes", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListRoutes(ctx, params)

			return err
		}},
		{"CreateSlice", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.CreateSliceOptions{
				Name: "second",
				Sst:  2,
			}

			return c.CreateSlice(ctx, opts)
		}},
		{"GetSlice", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetSlice(ctx, &client.GetSliceOptions{Name: "non-existent"})

			return err
		}},
		{"UpdateSlice", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateSliceOptions{
				Sst: 2,
			}

			return c.UpdateSlice(ctx, "non-existent", opts)
		}},
		{"DeleteSlice", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: "default"})
		}},
		{"ListSlices", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListSlices(ctx, params)

			return err
		}},
		{"GetStatus", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetStatus(ctx)

			return err
		}},
		{"CreateSubscriber", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createSubscriberOpts := &client.CreateSubscriberOptions{
				Imsi:           "invalid_imsi",
				Key:            "5122250214c33e723a5dd523fc145fc0",
				SequenceNumber: "000000000022",
				ProfileName:    "default",
			}

			return c.CreateSubscriber(ctx, createSubscriberOpts)
		}},
		{"GetSubscriber", func(t *testing.T, ctx context.Context, c *client.Client) error {
			imsi := "non_existent_imsi"

			getSubOpts := &client.GetSubscriberOptions{
				ID: imsi,
			}

			_, err := c.GetSubscriber(ctx, getSubOpts)

			return err
		}},
		{"UpdateSubscriber", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.UpdateSubscriberOptions{
				ProfileName: "enterprise",
			}

			return c.UpdateSubscriber(ctx, "non_existent_imsi", opts)
		}},
		{"DeleteSubscriber", func(t *testing.T, ctx context.Context, c *client.Client) error {
			imsi := "non_existent_imsi"

			deleteSubOpts := &client.DeleteSubscriberOptions{
				ID: imsi,
			}

			return c.DeleteSubscriber(ctx, deleteSubOpts)
		}},
		{"ListSubscribers", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListSubscribersParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListSubscribers(ctx, params)

			return err
		}},
		{"GetSubscriberCredentials", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.GetSubscriberCredentialsOptions{
				ID: "non_existent_imsi",
			}

			_, err := c.GetSubscriberCredentials(ctx, opts)

			return err
		}},
		{"GenerateSupportBundle", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.GenerateSupportBundleParams{
				Path: filepath.Join(t.TempDir(), "ella_support.tar.gz"),
			}

			return c.GenerateSupportBundle(ctx, params)
		}},
		{"ListUsage", func(t *testing.T, ctx context.Context, c *client.Client) error {
			params := &client.ListUsageParams{
				Start:      "2023-10-01",
				End:        "2023-10-02",
				GroupBy:    "day",
				Subscriber: "",
			}

			_, err := c.ListUsage(ctx, params)

			return err
		}},
		{"GetUsageRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetUsageRetentionPolicy(ctx)

			return err
		}},
		{"UpdateUsageRetentionPolicy", func(t *testing.T, ctx context.Context, c *client.Client) error {
			updateOpts := &client.UpdateUsageRetentionPolicyOptions{
				Days: -10,
			}

			return c.UpdateUsageRetentionPolicy(ctx, updateOpts)
		}},
		{"ClearUsage", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.ClearUsage(ctx)
		}},
		{"CreateUser", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createUserOpts := &client.CreateUserOptions{
				Email:    "invalid-email",
				Password: "secret",
			}

			return c.CreateUser(ctx, createUserOpts)
		}},
		{"ListUsers", func(t *testing.T, ctx context.Context, c *client.Client) error {
			listUsersParams := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListUsers(ctx, listUsersParams)

			return err
		}},
		{"DeleteUser", func(t *testing.T, ctx context.Context, c *client.Client) error {
			deleteUserOpts := &client.DeleteUserOptions{
				Email: "invalid-email",
			}

			return c.DeleteUser(ctx, deleteUserOpts)
		}},
		{"GetUser", func(t *testing.T, ctx context.Context, c *client.Client) error {
			_, err := c.GetUser(ctx, &client.GetUserOptions{Email: "missing@example.com"})

			return err
		}},
		{"UpdateUser", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.UpdateUser(ctx, "missing@example.com", &client.UpdateUserOptions{RoleID: client.RoleAdmin})
		}},
		{"CreateMyAPIToken", func(t *testing.T, ctx context.Context, c *client.Client) error {
			createAPITokenOpts := &client.CreateAPITokenOptions{
				Name:      "",
				ExpiresAt: "",
			}

			_, err := c.CreateMyAPIToken(ctx, createAPITokenOpts)

			return err
		}},
		{"DeleteMyAPIToken", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteMyAPIToken(ctx, "non-existent-token-id")
		}},
		{"ListMyAPITokens", func(t *testing.T, ctx context.Context, c *client.Client) error {
			param := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListMyAPITokens(ctx, param)

			return err
		}},
		{"ListUserAPITokens", func(t *testing.T, ctx context.Context, c *client.Client) error {
			param := &client.ListParams{
				Page:    1,
				PerPage: 10,
			}

			_, err := c.ListUserAPITokens(ctx, "nonexistent@example.com", param)

			return err
		}},
		{"CreateUserAPIToken", func(t *testing.T, ctx context.Context, c *client.Client) error {
			opts := &client.CreateAPITokenOptions{
				Name: "ab",
			}

			_, err := c.CreateUserAPIToken(ctx, "user@example.com", opts)

			return err
		}},
		{"DeleteUserAPIToken", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.DeleteUserAPIToken(ctx, "user@example.com", "nonexistent-id")
		}},
		{"UpdateMyPassword", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.UpdateMyPassword(ctx, &client.UpdateMyPasswordOptions{
				CurrentPassword: "wrongpass",
				Password:        "newpass",
			})
		}},
		{"UpdateUserPassword", func(t *testing.T, ctx context.Context, c *client.Client) error {
			return c.UpdateUserPassword(ctx, "nonexistent@example.com", &client.UpdateUserPasswordOptions{
				Password: "newpass",
			})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &client.Client{Requester: &fakeRequester{err: errRequester}}

			err := tt.call(t, context.Background(), c)
			if !errors.Is(err, errRequester) {
				t.Fatalf("error = %v, want it to wrap the requester failure", err)
			}
		})
	}
}
