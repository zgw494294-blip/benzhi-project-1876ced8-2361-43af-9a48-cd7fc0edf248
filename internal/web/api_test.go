package web

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/storage"
	"testing"
	"time"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	t.Cleanup(func() { service.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewHandler(service, logger))
	t.Cleanup(server.Close)
	return server
}
func TestIndexAndStrictJSON(t *testing.T) {
	server := testServer(t)
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Contains(body, []byte("吊挂安全放行工作台")) {
		t.Fatal("首页未交付完整工作台")
	}
	payload, _ := json.Marshal(map[string]any{"title": "演出", "venue": "剧场", "performanceAt": time.Now().Add(time.Hour), "operatorId": "op", "ruleSetVersion": "R1", "unknown": true})
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/rigging-sessions", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("未知字段状态码 = %d", response.StatusCode)
	}
}
