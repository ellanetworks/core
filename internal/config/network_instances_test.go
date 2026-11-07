// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func TestNetworkInstances(t *testing.T) {
	config.CheckInterfaceExistsFunc = func(name string) (bool, error) { return name != "missing", nil }
	config.GetInterfaceIPsFunc = func(name string) ([]string, error) { return []string{"1.2.3.4"}, nil }
	config.GetInterfaceIPFunc = func(name string, family config.AddressFamily) (string, error) { return "1.2.3.4", nil }
	config.GetVLANConfigForInterfaceFunc = func(name string) (*config.VlanConfig, error) {
		if name == "eth1.200" {
			return &config.VlanConfig{MasterInterface: "eth1", VlanId: 200}, nil
		}

		return nil, nil
	}
	config.VRFDeviceForInterfaceFunc = func(name string) (string, error) { return "", nil }

	const tmpl = `logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "test"
interfaces:
  n2:
    address: "0.0.0.0"
  n3:
    name: "eth0"
  n6:
    name: "eth2"
  network-instances:
%s
  api:
    address: "0.0.0.0"
    port: 5002
datapath:
  attach-mode: "tcx"
`

	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "two instances", yaml: "    - name: ims\n      n6:\n        name: n6-ims\n    - name: corp.example\n      n6:\n        name: eth1.200"},
		{name: "missing name", yaml: "    - n6:\n        name: n6-ims", wantErr: "name is empty"},
		{name: "invalid name", yaml: "    - name: IMS\n      n6:\n        name: n6-ims", wantErr: "not a valid network instance name"},
		{name: "reserved name", yaml: "    - name: default\n      n6:\n        name: n6-ims", wantErr: "network instance of interfaces.n6"},
		{name: "duplicate name", yaml: "    - name: ims\n      n6:\n        name: n6-ims\n    - name: ims\n      n6:\n        name: n6-mc", wantErr: "defined twice"},
		{name: "missing interface", yaml: "    - name: ims\n      n6:\n        name: missing", wantErr: "does not exist"},
		{name: "no interface", yaml: "    - name: ims", wantErr: "n6.name is empty"},
		{name: "N3 interface", yaml: "    - name: ims\n      n6:\n        name: eth0", wantErr: "already used by interfaces.n3"},
		{name: "default N6 interface", yaml: "    - name: ims\n      n6:\n        name: eth2", wantErr: "already used by interfaces.n6"},
		{name: "shared interface", yaml: "    - name: ims\n      n6:\n        name: n6-ims\n    - name: mc\n      n6:\n        name: n6-ims", wantErr: "already used by interfaces.network-instances[0]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/config.yaml"
			if err := os.WriteFile(path, []byte(strings.Replace(tmpl, "%s", tc.yaml, 1)), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			conf, err := config.Validate(path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %v, want one containing %q", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := conf.Interfaces.NetworkInstances
			if len(got) != 2 || got[0].Name != "ims" || got[0].N6.Name != "n6-ims" || got[0].N6.VlanConfig != nil ||
				got[1].Name != "corp.example" || got[1].N6.VlanConfig == nil || got[1].N6.VlanConfig.VlanId != 200 {
				t.Fatalf("network instances = %+v", got)
			}
		})
	}
}

func TestNetworkInstancesGenericVLANOfDatapathInterface(t *testing.T) {
	config.CheckInterfaceExistsFunc = func(name string) (bool, error) { return true, nil }
	config.GetInterfaceIPsFunc = func(name string) ([]string, error) { return []string{"1.2.3.4"}, nil }
	config.GetInterfaceIPFunc = func(name string, family config.AddressFamily) (string, error) { return "1.2.3.4", nil }
	config.GetVLANConfigForInterfaceFunc = func(name string) (*config.VlanConfig, error) {
		switch name {
		case "eth2.200":
			return &config.VlanConfig{MasterInterface: "eth2", VlanId: 200}, nil
		case "eth0.300":
			return &config.VlanConfig{MasterInterface: "eth0", VlanId: 300}, nil
		case "eth5.100":
			return &config.VlanConfig{MasterInterface: "eth5", VlanId: 100}, nil
		case "eth2.400":
			return &config.VlanConfig{MasterInterface: "eth2", VlanId: 400}, nil
		}

		return nil, nil
	}
	config.VRFDeviceForInterfaceFunc = func(name string) (string, error) { return "", nil }

	const tmpl = `logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "test"
interfaces:
  n2:
    address: "0.0.0.0"
  n3:
    name: "eth0"
  n6:
    name: "%s"
  network-instances:
%s
  api:
    address: "0.0.0.0"
    port: 5002
datapath:
  attach-mode: "%s"
`

	cases := []struct {
		name    string
		n6      string
		yaml    string
		mode    string
		wantErr string
	}{
		{name: "VLAN of N6", n6: "eth2", yaml: "    - name: corp\n      n6:\n        name: eth2.200", mode: "xdp-generic", wantErr: "is a VLAN of eth2, the interface of interfaces.n6"},
		{name: "VLAN of N3", n6: "eth2", yaml: "    - name: corp\n      n6:\n        name: eth0.300", mode: "xdp-generic", wantErr: "is a VLAN of eth0, the interface of interfaces.n3"},
		{name: "VLAN of another instance", n6: "eth2", yaml: "    - name: a\n      n6:\n        name: eth5\n    - name: b\n      n6:\n        name: eth5.100", mode: "xdp-generic", wantErr: "the interface of interfaces.network-instances[0].n6"},
		{name: "N6 is a VLAN of an instance", n6: "eth2.400", yaml: "    - name: corp\n      n6:\n        name: eth2", mode: "xdp-generic", wantErr: "interfaces.n6 (eth2.400) is a VLAN of eth2"},
		{name: "unrelated VLAN", n6: "eth2", yaml: "    - name: corp\n      n6:\n        name: eth5.100", mode: "xdp-generic"},
		{name: "VLAN of N6 at TCX", n6: "eth2", yaml: "    - name: corp\n      n6:\n        name: eth2.200", mode: "tcx"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/config.yaml"

			body := fmt.Sprintf(tmpl, tc.n6, tc.yaml, tc.mode)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := config.Validate(path)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}
