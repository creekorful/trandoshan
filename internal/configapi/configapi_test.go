package configapi

import (
	"github.com/creekorful/trandoshan/internal/configapi/api_mock"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetConfiguration(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	apiMock := api_mock.NewMockConfigAPI(mockCtrl)
	apiMock.EXPECT().Get("hello").Return([]byte("{\"ttl\": \"10s\"}"), nil)

	req := httptest.NewRequest(http.MethodGet, "/config/hello", nil)
	req = mux.SetURLVars(req, map[string]string{"key": "hello"})

	rec := httptest.NewRecorder()

	s := State{api: apiMock}
	s.getConfiguration(rec, req)

	if rec.Code != http.StatusOK {
		t.Fail()
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fail()
	}

	b, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		t.FailNow()
	}

	if string(b) != "{\"ttl\": \"10s\"}" {
		t.Fail()
	}
}

func TestSetConfiguration(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	apiMock := api_mock.NewMockConfigAPI(mockCtrl)
	apiMock.EXPECT().Set("hello", []byte("{\"ttl\": \"10s\"}")).Return(nil)

	req := httptest.NewRequest(http.MethodPut, "/config/hello", strings.NewReader("{\"ttl\": \"10s\"}"))
	req = mux.SetURLVars(req, map[string]string{"key": "hello"})

	rec := httptest.NewRecorder()

	s := State{api: apiMock}
	s.setConfiguration(rec, req)

	if rec.Code != http.StatusOK {
		t.Fail()
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fail()
	}

	b, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		t.FailNow()
	}

	if string(b) != "{\"ttl\": \"10s\"}" {
		t.Fail()
	}
}
