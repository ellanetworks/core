// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
)

type N2Interface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Interface string   `json:"interface"`
}

type N3Interface struct {
	Name            string   `json:"name"`
	Addresses       []string `json:"addresses"`
	ExternalAddress string   `json:"external_address"`
}

type N6Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

type APIInterface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
}

type DiameterInterface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Interface string   `json:"interface"`
}

type N32Interface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Interface string   `json:"interface"`
}

type NetworkInterfaces struct {
	N2       N2Interface        `json:"n2"`
	N3       N3Interface        `json:"n3"`
	N6       N6Interface        `json:"n6"`
	API      APIInterface       `json:"api"`
	Diameter *DiameterInterface `json:"diameter"`
	N32      *N32Interface      `json:"n32"`
}

type GetNetworkInterfaceInfoResponse struct {
	Result NetworkInterfaces `json:"result"`
	Error  string            `json:"error,omitempty"`
}

type UpdateN3InfoParams struct {
	ExternalAddress string `json:"external_address"`
}

type UpdateN3InfoResponseResult struct {
	Message string `json:"message"`
}

type UpdateN3InfoResponse struct {
	Result UpdateN3InfoResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

func listNetworkInterfaces(url string, client *http.Client, token string) (int, *GetNetworkInterfaceInfoResponse, error) {
	return apiDo[GetNetworkInterfaceInfoResponse](client, "GET", url+"/api/v1/networking/interfaces", token, nil)
}

func updateN3Info(url string, client *http.Client, token string, externalAddress string) (int, *UpdateN3InfoResponse, error) {
	params := UpdateN3InfoParams{
		ExternalAddress: externalAddress,
	}

	return apiDo[UpdateN3InfoResponse](client, "PUT", url+"/api/v1/networking/interfaces/n3", token, params)
}

func TestNetworkInteraces_EndToEnd(t *testing.T) {
	originalGetInterfaceIPsFunc := config.GetInterfaceIPsFunc
	config.GetInterfaceIPsFunc = func(name string) ([]string, error) {
		switch name {
		case "eth1":
			return []string{"14.14.14.14"}, nil
		default:
			return nil, nil
		}
	}

	t.Cleanup(func() {
		config.GetInterfaceIPsFunc = originalGetInterfaceIPsFunc
	})

	env, client, token := newAuthedTestEnv(t)

	t.Run("1. List network interfaces", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N2.Addresses[0] != "12.12.12.12" {
			t.Fatalf("unexpected N2 interface address: %v", resp.Result.N2.Addresses)
		}

		if resp.Result.N2.Port != 2152 {
			t.Fatalf("unexpected N2 interface port: %d", resp.Result.N2.Port)
		}

		if resp.Result.N3.Name != "" {
			t.Fatalf("unexpected N3 interface name: %s", resp.Result.N3.Name)
		}

		if resp.Result.N3.Addresses[0] != "13.13.13.13" {
			t.Fatalf("unexpected N3 interface address: %v", resp.Result.N3.Addresses)
		}

		if resp.Result.N3.ExternalAddress != "" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}

		if resp.Result.N6.Name != "eth1" {
			t.Fatalf("unexpected N6 interface name: %s", resp.Result.N6.Name)
		}

		if len(resp.Result.N6.Addresses) != 1 || resp.Result.N6.Addresses[0] != "14.14.14.14" {
			t.Fatalf("unexpected N6 interface addresses: %v", resp.Result.N6.Addresses)
		}

		if len(resp.Result.API.Addresses) != 0 {
			t.Fatalf("unexpected API interface addresses: %v", resp.Result.API.Addresses)
		}

		if resp.Result.API.Port != 8443 {
			t.Fatalf("unexpected API interface port: %d", resp.Result.API.Port)
		}

		if d := resp.Result.Diameter; d == nil || len(d.Addresses) != 1 || d.Addresses[0] != "15.15.15.15" || d.Port != 3868 || d.Interface != "" {
			t.Fatalf("unexpected Diameter interface: %+v", d)
		}

		if resp.Result.N32 != nil {
			t.Fatalf("expected no N32 interface, got %+v", resp.Result.N32)
		}
	})

	t.Run("2. Update N3 external address to IP", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(env.Server.URL, client, token, "192.168.1.1")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "N3 interface updated" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("3. Validate that N3 external address is updated", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N3.ExternalAddress != "192.168.1.1" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}
	})

	t.Run("4. Update N3 external address to FQDN", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(env.Server.URL, client, token, "example.com")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if updateResponse.Error != "Invalid external address. Must be an IPv4 address, an IPv6 address, or an IPv4 and an IPv6 address separated by a comma" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}
	})

	t.Run("5. Update N3 external address to empty", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(env.Server.URL, client, token, "")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "N3 interface updated" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("6. Validate that N3 external address is updated", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N3.ExternalAddress != "" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}
	})

	t.Run("7. Update N3 external address to IPv6", func(t *testing.T) {
		statusCode, updateResponse, err := updateN3Info(env.Server.URL, client, token, "2001:db8::1")
		if err != nil {
			t.Fatalf("couldn't update N3 info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "N3 interface updated" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("8. Validate that N3 external IPv6 address is updated", func(t *testing.T) {
		statusCode, resp, err := listNetworkInterfaces(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list network interfaces: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if resp.Error != "" {
			t.Fatalf("expected no error, got %s", resp.Error)
		}

		if resp.Result.N3.ExternalAddress != "2001:db8::1" {
			t.Fatalf("unexpected N3 interface external address: %s", resp.Result.N3.ExternalAddress)
		}
	})
}

func newN32InterfaceTestEnv(t *testing.T, n32 config.N32Interface) (testEnv, *http.Client, string) {
	t.Helper()

	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}

	env, err := buildTestEnvWith(testdb, nil, nil, func(c *server.HandlerConfig) {
		c.Config.Interfaces.N32 = n32
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(env.Server.Close)

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatal(err)
	}

	return env, client, token
}

func TestNetworkInterfaces_N32(t *testing.T) {
	originalGetInterfaceIPsFunc := config.GetInterfaceIPsFunc
	config.GetInterfaceIPsFunc = func(name string) ([]string, error) {
		switch name {
		case "ens5":
			return []string{"16.16.16.16", "2001:db8::16"}, nil
		default:
			return nil, nil
		}
	}

	t.Cleanup(func() {
		config.GetInterfaceIPsFunc = originalGetInterfaceIPsFunc
	})

	tests := []struct {
		name          string
		n32           config.N32Interface
		wantAddresses []string
		wantPort      int
		wantInterface string
	}{
		{"address", config.N32Interface{Enabled: true, Address: "10.0.0.32", Port: 443}, []string{"10.0.0.32"}, 443, ""},
		{"interface name", config.N32Interface{Enabled: true, Name: "ens5", Port: 8443}, []string{"16.16.16.16", "2001:db8::16"}, 8443, "ens5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, client, token := newN32InterfaceTestEnv(t, tc.n32)

			statusCode, resp, err := listNetworkInterfaces(env.Server.URL, client, token)
			if err != nil {
				t.Fatalf("couldn't list network interfaces: %s", err)
			}

			if statusCode != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
			}

			n := resp.Result.N32
			if n == nil {
				t.Fatal("expected an N32 interface")
			}

			if len(n.Addresses) != len(tc.wantAddresses) {
				t.Fatalf("unexpected N32 addresses: %v", n.Addresses)
			}

			for i, a := range tc.wantAddresses {
				if n.Addresses[i] != a {
					t.Fatalf("unexpected N32 addresses: %v", n.Addresses)
				}
			}

			if n.Port != tc.wantPort {
				t.Fatalf("unexpected N32 port: %d", n.Port)
			}

			if n.Interface != tc.wantInterface {
				t.Fatalf("unexpected N32 interface: %q", n.Interface)
			}
		})
	}
}
