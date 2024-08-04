package configapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/webui/configmodels"
	"github.com/yeastengine/ella/internal/webui/dbadapter"
	"go.mongodb.org/mongo-driver/bson"
)

type MockMongoClientNoGnbs struct {
	dbadapter.DBInterface
}

type MockMongoClientOneGnb struct {
	dbadapter.DBInterface
}

type MockMongoClientManyGnbs struct {
	dbadapter.DBInterface
}

func (m *MockMongoClientNoGnbs) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	return results, nil
}

func (m *MockMongoClientOneGnb) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	gnb := configmodels.Gnb{
		Name: "gnb1",
		Tac:  "123",
	}
	var gnbBson bson.M
	tmp, _ := json.Marshal(gnb)
	json.Unmarshal(tmp, &gnbBson)

	results = append(results, gnbBson)
	return results, nil
}

var mockConfigChannel chan *configmodels.ConfigMessage

func (m *MockMongoClientManyGnbs) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	names := []string{"gnb0", "gnb1", "gnb2"}
	tacs := []string{"12", "345", "678"}
	for i, name := range names {
		gnb := configmodels.Gnb{
			Name: name,
			Tac:  tacs[i],
		}
		var gnbBson bson.M
		tmp, _ := json.Marshal(gnb)
		json.Unmarshal(tmp, &gnbBson)

		results = append(results, gnbBson)
	}
	return results, nil
}

func TestGivenNoGnbsWhenGetGnbsThenReturnsAnEmptyList(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	dbadapter.CommonDBClient = &MockMongoClientNoGnbs{}

	GetGnbs(c)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	if body != "[]" {
		t.Errorf("Expected empty JSON list, got %v", body)
	}
}

func TestGivenOneGnbWhenGetGnbsThenReturnsAListWithOneElement(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	dbadapter.CommonDBClient = &MockMongoClientOneGnb{}

	GetGnbs(c)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `[{"name":"gnb1","tac":"123"}]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestGivenManyGnbsWhenGetGnbsThenReturnsAListWithManyGnbs(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	dbadapter.CommonDBClient = &MockMongoClientManyGnbs{}

	GetGnbs(c)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `[{"name":"gnb0","tac":"12"},{"name":"gnb1","tac":"345"},{"name":"gnb2","tac":"678"}]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestPostGnbByName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("gNB TAC is not a string", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "gnb-name", Value: "test-gnb"}}
		req, _ := http.NewRequest(http.MethodPost, "/gnb", strings.NewReader(`{"tac": 1234}`))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		PostGnb(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected StatusCode %d, got %d", http.StatusBadRequest, w.Code)
		}
		expectedError := `{"error":"Failed to create gNB test-gnb: json: cannot unmarshal number into Go struct field Gnb.tac of type string"}`
		if w.Body.String() != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, w.Body.String())
		}
	})

	t.Run("Missing TAC in JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "gnb-name", Value: "test-gnb"}}
		req, _ := http.NewRequest(http.MethodPost, "/gnb", strings.NewReader(`{"some_param": "123"}`))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		PostGnb(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected StatusCode %d, got %d", http.StatusBadRequest, w.Code)
		}

		expectedError := `{"error":"Post gNB request body is missing tac"}`
		if w.Body.String() != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, w.Body.String())
		}
	})

	t.Run("Missing gNB name", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodPost, "/gnb", nil)
		c.Request = req

		PostGnb(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected StatusCode %d, got %d", http.StatusBadRequest, w.Code)
		}
		expectedError := `{"error":"Post gNB request is missing gnb-name"}`
		if w.Body.String() != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, w.Body.String())
		}
	})
}

func TestDeleteGnbByName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Missing gNB name", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodPost, "/gnb", nil)
		c.Request = req

		DeleteGnb(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected StatusCode %d, got %d", http.StatusBadRequest, w.Code)
		}
		expectedError := `{"error":"Delete gNB request is missing gnb-name"}`
		if w.Body.String() != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, w.Body.String())
		}
	})
}
