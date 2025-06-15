package httpclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bignyap/go-utilities/httpclient"
)

type TestMessage struct {
	Text string `json:"text"`
}

type TestResponse struct {
	Status string `json:"status"`
}

func TestPost_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg TestMessage
		_ = json.NewDecoder(r.Body).Decode(&msg)

		if msg.Text != "hello" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		resp := TestResponse{Status: "ok"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := httpclient.NewHystixClient(server.URL, httpclient.ClientConfig{}, nil)

	var res TestResponse
	err := client.Post("/test", TestMessage{Text: "hello"}, &res)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Status != "ok" {
		t.Errorf("expected status ok, got %s", res.Status)
	}
}
