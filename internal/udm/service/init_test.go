package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yeastengine/ella/internal/udm/context"
)

func Test_nrf_url_is_not_overwritten_when_registering(t *testing.T) {
	svr := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "banana")
		if err != nil {
			t.Errorf("Fprintf failed to format/write. Error: %v", err)
		}
	}))
	svr.EnableHTTP2 = true
	svr.StartTLS()
	defer svr.Close()
	self := context.UDM_Self()
	self.NrfUri = svr.URL
	self.RegisterIPv4 = "127.0.0.2"
	var udm *UDM
	go udm.registerNF()
	ConfigPodTrigger <- true

	time.Sleep(1 * time.Second)
	if self.NrfUri != svr.URL {
		t.Errorf("Expected NRF URL to stay %v, but was %v", svr.URL, self.NrfUri)
	}
}
