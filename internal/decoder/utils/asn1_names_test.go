// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var updateNames = flag.Bool("update", false, "regenerate asn1_names.go from the vendored ASN.1 modules")

var (
	componentRe = regexp.MustCompile(`(?m)^\s+([a-z][A-Za-z0-9-]*)\s+[A-Z]`)
	typeRe      = regexp.MustCompile(`(?m)^([A-Z][A-Za-z0-9-]*)\s*::=`)
)

func TestASN1NamesUpToDate(t *testing.T) {
	var modules []string

	for _, dir := range []string{"../../../ngap/testdata/asn1", "../../../s1ap/testdata/asn1"} {
		paths, err := filepath.Glob(filepath.Join(dir, "*.asn"))
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) == 0 {
			t.Fatalf("no ASN.1 modules under %s", dir)
		}

		modules = append(modules, paths...)
	}

	sort.Strings(modules)

	components := map[string]map[string]int{}
	types := map[string]map[string]int{}

	for _, p := range modules {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range componentRe.FindAllSubmatch(b, -1) {
			count(components, string(m[1]))
		}

		for _, m := range typeRe.FindAllSubmatch(b, -1) {
			count(types, string(m[1]))
		}
	}

	names := map[string]string{}

	for key, forms := range types {
		names[key] = asn1Snake(mostCommon(forms))
	}

	for key, forms := range components {
		names[key] = asn1Snake(mostCommon(forms))
	}

	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var buf bytes.Buffer

	buf.WriteString("// SPDX-FileCopyrightText: Ella Networks Inc.\n// SPDX-License-Identifier: BUSL-1.1\n\npackage utils\n\nvar asn1Names = map[string]string{ // #nosec: G101 -- ASN.1 identifier names, not credentials\n")

	for _, k := range keys {
		fmt.Fprintf(&buf, "\t%q: %q,\n", k, names[k])
	}

	buf.WriteString("}\n")

	want, err := format.Source(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if *updateNames {
		if err := os.WriteFile("asn1_names.go", want, 0o644); err != nil {
			t.Fatal(err)
		}

		return
	}

	got, err := os.ReadFile("asn1_names.go")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Error("asn1_names.go is stale: run go test ./internal/decoder/utils/ -run TestASN1NamesUpToDate -update")
	}
}

func count(into map[string]map[string]int, ident string) {
	key := strings.ToLower(strings.ReplaceAll(ident, "-", ""))
	if into[key] == nil {
		into[key] = map[string]int{}
	}

	into[key][ident]++
}

func mostCommon(forms map[string]int) string {
	best, n := "", 0

	for f, c := range forms {
		if c > n || (c == n && f < best) {
			best, n = f, c
		}
	}

	return best
}

func asn1Snake(ident string) string {
	segments := strings.Split(ident, "-")
	out := make([]string, 0, len(segments))

	for i, s := range segments {
		if s == "" {
			continue
		}

		if i == 0 {
			s = strings.ToUpper(s[:1]) + s[1:]
		}

		out = append(out, SnakeCase(s))
	}

	return strings.Join(out, "_")
}

func TestFieldName(t *testing.T) {
	cases := map[string]string{
		"GTPTEID":               "gtp_teid",
		"TransportLayerAddress": "transport_layer_address",
		"EUTRApagingeDRXCycle":  "eutra_paging_e_drx_cycle",
		"Pc5QoSFlowList":        "pc5_qos_flow_list",
	}

	for in, want := range cases {
		if got := FieldName(in); got != want {
			t.Errorf("FieldName(%q) = %q, want %q", in, got, want)
		}
	}
}
