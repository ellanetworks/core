// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package redact

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/config"
	yamlv2 "gopkg.in/yaml.v2"
)

func TestApplyYAMLMasksJoinToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"plain", "cluster:\n  join-token: live-secret-token\n"},
		{"quoted", "cluster:\n  \"join-token\": \"live-secret-token\"\n"},
		{"flow", "cluster: {join-token: live-secret-token}\n"},
		{"folded", "cluster:\n  join-token: >-\n    live-secret-token\n"},
		{"literal", "cluster:\n  join-token: |-\n    live-secret-token\n"},
		{"value alias", "db:\n  path: &token live-secret-token\ncluster:\n  join-token: *token\n"},
		{"key alias", "db:\n  path: &key join-token\ncluster:\n  *key: live-secret-token\n"},
		{"binary key", "cluster:\n  !!binary am9pbi10b2tlbg==: live-secret-token\n"},
		{"binary value", "cluster:\n  join-token: !!binary bGl2ZS1zZWNyZXQtdG9rZW4=\n"},
		{"merge", "cluster:\n  <<: &defaults\n    join-token: live-secret-token\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before config.ConfigYAML
			if err := yamlv2.UnmarshalStrict([]byte(tc.input), &before); err != nil {
				t.Fatal(err)
			}

			if before.Cluster.JoinToken != "live-secret-token" {
				t.Fatalf("runtime parser did not recognize token: %q", before.Cluster.JoinToken)
			}

			out, err := ApplyYAML([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}

			for _, secret := range []string{"live-secret-token", "bGl2ZS1zZWNyZXQtdG9rZW4="} {
				if bytes.Contains(out, []byte(secret)) {
					t.Fatalf("credential retained: %s", out)
				}
			}

			var after config.ConfigYAML
			if err := yamlv2.UnmarshalStrict(out, &after); err != nil {
				t.Fatal(err)
			}

			if after.Cluster.JoinToken != "*" {
				t.Fatalf("token not masked: %q", after.Cluster.JoinToken)
			}
		})
	}
}

func TestApplyYAMLMaskPropagatesThroughAliasesAndMerges(t *testing.T) {
	for _, input := range []string{
		"cluster:\n  join-token: &token live-secret-token\nother: *token\n",
		"defaults: &defaults {join-token: live-secret-token}\ncluster: {<<: *defaults}\n",
		"defaults: &defaults {cluster: {join-token: live-secret-token}}\n<<: *defaults\n",
		"one: &one {join-token: live-secret-token}\ntwo: &two {node-id: 1}\ncluster: {<<: [*one, *two]}\n",
	} {
		out, err := ApplyYAML([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Contains(out, []byte("live-secret-token")) {
			t.Fatalf("aliased credential retained: %s", out)
		}

		var got struct {
			Cluster struct {
				JoinToken string `yaml:"join-token"`
			} `yaml:"cluster"`
			Other string `yaml:"other"`
		}
		if err := yamlv2.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}

		if got.Cluster.JoinToken != "*" || (got.Other != "" && got.Other != "*") {
			t.Fatalf("alias or merge did not resolve to mask: %#v", got)
		}
	}
}

func TestApplyYAMLRetainsOperationalData(t *testing.T) {
	in := []byte("cluster:\n  join-token: live-secret-token\n  advertise-address: 10.0.0.1\n  node-id: 1\ninterfaces:\n  api:\n    tls:\n      key: /etc/ella/key.pem\n")
	want := []byte("cluster:\n  join-token: \"*\"\n  advertise-address: 10.0.0.1\n  node-id: 1\ninterfaces:\n  api:\n    tls:\n      key: /etc/ella/key.pem\n")

	out, err := ApplyYAML(in)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, want) {
		t.Fatalf("unexpected configuration:\n%s\nwant:\n%s", out, want)
	}
}

func TestApplyYAMLFailsClosed(t *testing.T) {
	for _, input := range []string{
		"cluster:\n  join-token: \"unterminated-secret\n",
		"cluster:\n  join-token: [live-secret-token]\n",
		"cluster:\n  join-token: {value: live-secret-token}\n",
		"cluster:\n  ? [join-token]\n  : live-secret-token\n",
		"cluster:\n  !!binary invalid-base64: live-secret-token\n",
	} {
		out, err := ApplyYAML([]byte(input))
		if err == nil || out != nil {
			t.Fatalf("unsafe configuration accepted: %s", out)
		}

		if bytes.Contains([]byte(err.Error()), []byte("secret")) {
			t.Fatalf("error contains credential: %v", err)
		}
	}
}
