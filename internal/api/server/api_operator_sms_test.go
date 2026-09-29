// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

type UpdateOperatorSMSParams struct {
	SMSCAddress string `json:"smscAddress"`
	SMSCPort    int    `json:"smscPort,omitempty"`
	SMSNumber   string `json:"smsNumber"`
}

type UpdateOperatorSMSResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateOperatorSMS(url string, client *http.Client, token string, data *UpdateOperatorSMSParams) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "PUT", url+"/api/v1/operator/sms", token, data)
}

func TestUpdateOperatorSMS(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	getSMS := func(t *testing.T) GetOperatorSMSResponseResult {
		t.Helper()

		code, resp, err := getOperator(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("get operator: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("get operator: expected 200, got %d (%q)", code, resp.Error)
		}

		return resp.Result.SMS
	}

	t.Run("SMS is disabled by default", func(t *testing.T) {
		got := getSMS(t)
		want := GetOperatorSMSResponseResult{SMSCPort: 3868}

		if got != want {
			t.Fatalf("sms = %+v, want %+v", got, want)
		}
	})

	t.Run("setting an SMSC enables SMS", func(t *testing.T) {
		code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{
			SMSCAddress: "192.0.2.10",
			SMSNumber:   "+15550001111",
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusCreated {
			t.Fatalf("update: expected 201, got %d (%q)", code, resp.Error)
		}

		got := getSMS(t)
		want := GetOperatorSMSResponseResult{Enabled: true, SMSCAddress: "192.0.2.10", SMSCPort: 3868, SMSNumber: "+15550001111"}

		if got != want {
			t.Fatalf("sms = %+v, want %+v", got, want)
		}
	})

	t.Run("a custom port is stored", func(t *testing.T) {
		code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{
			SMSCAddress: "2001:db8::10",
			SMSCPort:    3869,
			SMSNumber:   "+15550001111",
		})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getSMS(t); got.SMSCAddress != "2001:db8::10" || got.SMSCPort != 3869 {
			t.Fatalf("sms = %+v", got)
		}
	})

	t.Run("invalid settings are rejected", func(t *testing.T) {
		cases := map[string]UpdateOperatorSMSParams{
			"hostname":       {SMSCAddress: "smsc.example.org", SMSNumber: "+15550001111"},
			"missing number": {SMSCAddress: "192.0.2.10"},
			"bad number":     {SMSCAddress: "192.0.2.10", SMSNumber: "+0555"},
			"no plus sign":   {SMSCAddress: "192.0.2.10", SMSNumber: "15550001111"},
			"bad port":       {SMSCAddress: "192.0.2.10", SMSCPort: 70000, SMSNumber: "+15550001111"},
		}

		for name, params := range cases {
			code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &params)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", name, code, resp.Error)
			}
		}

		if got := getSMS(t); got.SMSCAddress != "2001:db8::10" {
			t.Fatalf("rejected updates changed the settings: %+v", got)
		}
	})

	t.Run("clearing the SMSC disables SMS", func(t *testing.T) {
		code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{SMSNumber: "+15550001111"})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		got := getSMS(t)
		want := GetOperatorSMSResponseResult{SMSCPort: 3868, SMSNumber: "+15550001111"}

		if got != want {
			t.Fatalf("sms = %+v, want %+v", got, want)
		}
	})
}
