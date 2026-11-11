// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestNamesInPathsReachTheServerUnchanged(t *testing.T) {
	var got []string

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/integrations/sms/smscs/{name}", func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.PathValue("name"))
		_, _ = w.Write([]byte(`{"result":{}}`))
	})

	for _, route := range []string{
		"GET /api/v1/integrations/ims/application-servers/{name}",
		"GET /api/v1/profiles/{name}",
		"GET /api/v1/networking/data-networks/{name}",
	} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			got = append(got, r.PathValue("name"))
			_, _ = w.Write([]byte(`{"result":{}}`))
		})
	}

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := client.New(&client.Config{BaseURLs: []string{srv.URL}})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"a/b?c #(2)", "50%", "a%20b"} {
		got = nil

		if _, err := c.GetSMSC(context.Background(), name); err != nil {
			t.Fatalf("GetSMSC(%q): %v", name, err)
		}

		if _, err := c.GetIMSApplicationServer(context.Background(), name); err != nil {
			t.Fatalf("GetIMSApplicationServer(%q): %v", name, err)
		}

		if _, err := c.GetProfile(context.Background(), &client.GetProfileOptions{Name: name}); err != nil {
			t.Fatalf("GetProfile(%q): %v", name, err)
		}

		if _, err := c.GetDataNetwork(context.Background(), &client.GetDataNetworkOptions{Name: name}); err != nil {
			t.Fatalf("GetDataNetwork(%q): %v", name, err)
		}

		if want := []string{name, name, name, name}; !slices.Equal(got, want) {
			t.Fatalf("the server received %q, want %q", got, want)
		}
	}
}
