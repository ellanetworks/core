// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestNamesInPathsReachTheServerUnchanged(t *testing.T) {
	var got []string

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/operator/sms/smscs/{name}", func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.PathValue("name"))
		_, _ = w.Write([]byte(`{"result":{}}`))
	})
	mux.HandleFunc("GET /api/v1/operator/ims/application-servers/{name}", func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.PathValue("name"))
		_, _ = w.Write([]byte(`{"result":{}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := client.New(&client.Config{BaseURLs: []string{srv.URL}})
	if err != nil {
		t.Fatal(err)
	}

	const name = "a/b?c #(2)"

	if _, err := c.GetSMSC(context.Background(), name); err != nil {
		t.Fatalf("GetSMSC: %v", err)
	}

	if _, err := c.GetIMSApplicationServer(context.Background(), name); err != nil {
		t.Fatalf("GetIMSApplicationServer: %v", err)
	}

	if len(got) != 2 || got[0] != name || got[1] != name {
		t.Fatalf("the server received %q, want %q twice", got, name)
	}
}
