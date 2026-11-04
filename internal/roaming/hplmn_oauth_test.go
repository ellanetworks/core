// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func TestTokensAreScopedByProducerAndServiceNotByURL(t *testing.T) {
	var issued atomic.Int32

	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wire.AccessTokenPath() {
			_ = r.ParseForm()
			if r.Form.Get("scope") != "nausf-auth" || r.Form.Get("targetNfType") != "AUSF" || r.Form.Get("nfType") != "AMF" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			issued.Add(1)
			w.Header().Set("Content-Type", wire.ContentTypeJSON)
			_, _ = w.Write([]byte(`{"access_token": "t", "token_type": "Bearer", "expires_in": 3600}`))

			return
		}

		if r.Header.Get("Authorization") != "Bearer t" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="nausf-auth"`)
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.WriteHeader(http.StatusOK)
	})}

	sbi := NewHPLMNSBI(f, NewDiscovery(f))
	target := HPLMNTarget{PartnerID: "p", PLMN: home, Serving: serving}
	req := HPLMNRequest{Method: http.MethodPut, Requester: wire.NFTypeAMF, Producer: wire.NFTypeAUSF, Service: wire.ServiceNausfAuth, Accept: []int{http.StatusOK}}

	for _, url := range []string{"http://10.0.0.1:8080/nausf-auth/v1/ue-authentications/1/5g-aka-confirmation", "http://ausf-2.internal/nausf-auth/v1/ue-authentications/2/5g-aka-confirmation"} {
		req.URL = url
		if _, err := sbi.Do(context.Background(), target, req); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}

	if issued.Load() != 1 {
		t.Fatalf("%d tokens issued, want one for every nausf-auth URI", issued.Load())
	}

	unscoped := req
	unscoped.Producer, unscoped.Service = "", ""

	_, err := sbi.Do(context.Background(), target, unscoped)
	if pd, ok := ProblemOf(err); !ok || pd.Status != http.StatusUnauthorized || issued.Load() != 1 {
		t.Fatalf("unscoped request: %v, %d tokens", err, issued.Load())
	}

	sdm := req
	sdm.Producer, sdm.Service = wire.NFTypeUDM, wire.ServiceNudmSDM

	if _, err := sbi.Do(context.Background(), target, sdm); !errors.Is(err, ErrHPLMNUnauthorized) || !strings.Contains(err.Error(), "400") {
		t.Fatalf("a nudm-sdm request reused the nausf-auth token: %v", err)
	}
}
