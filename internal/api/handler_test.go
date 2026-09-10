package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/api"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
)

type stubES struct {
	addUser, addSeg string
	count           int64
	mode            config.Mode
}

func (s *stubES) Add(_ context.Context, userID, segment string) error {
	s.addUser, s.addSeg = userID, segment
	return nil
}
func (s *stubES) Count(_ context.Context, _ string) (int64, error) { return s.count, nil }
func (s *stubES) Mode(_ string) config.Mode {
	if s.mode == "" {
		return config.ModeApproximate
	}
	return s.mode
}

func TestAddAndCountHandlers(t *testing.T) {
	es := &stubES{count: 7, mode: config.ModeExact}
	h := api.NewHandler(es)
	mux := http.NewServeMux()
	h.Routes(mux)

	body := []byte(`{"user_id":"u104010","segment":"sports"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("add status=%d body=%s", rec.Code, rec.Body.String())
	}
	if es.addUser != "u104010" || es.addSeg != "sports" {
		t.Fatalf("add not recorded: %+v", es)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/segments/sports/count", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("count status=%d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if int64(resp["count"].(float64)) != 7 {
		t.Fatalf("count resp=%v", resp)
	}
}
