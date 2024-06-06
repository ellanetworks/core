/*
 *  Tests for NSSF Configuration Factory
 */

package factory

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Webui URL is not set then default Webui URL value is returned
func TestGetDefaultWebuiUrl(t *testing.T) {
	if err := InitConfigFactory("../test/conf/test_nssf_config.yaml"); err != nil {
		fmt.Printf("Error in InitConfigFactory: %v\n", err)
	}
	got := NssfConfig.Configuration.WebuiUri
	want := "webui:9876"
	assert.Equal(t, got, want, "The webui URL is not correct.")
}

// Webui URL is set to a custom value then custom Webui URL is returned
func TestGetCustomWebuiUrl(t *testing.T) {
	if err := InitConfigFactory("../test/conf/test_nssf_config_with_custom_webui_url.yaml"); err != nil {
		fmt.Printf("Error in InitConfigFactory: %v\n", err)
	}
	got := NssfConfig.Configuration.WebuiUri
	want := "myspecialwebui:9872"
	assert.Equal(t, got, want, "The webui URL is not correct.")
}
