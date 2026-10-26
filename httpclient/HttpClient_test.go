package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestDoRequestWithContext_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewHystixClient(server.URL, httpclient.ClientConfig{
		RetryCount: 1,
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.DoRequestWithContext(ctx, http.MethodGet, "/slow", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
}

func TestNewTracingHTTPClient_Pooled(t *testing.T) {
	client := httpclient.NewTracingHTTPClient(5 * time.Second)
	if client == nil {
		t.Fatal("expected non-nil tracing HTTP client")
	}
	tr, ok := client.Transport.(*httpclient.TracingRoundTripper)
	if !ok {
		t.Fatal("expected TracingRoundTripper")
	}
	baseTransport, ok := tr.Base.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport as base")
	}
	if baseTransport.MaxIdleConnsPerHost != 50 {
		t.Errorf("expected MaxIdleConnsPerHost to be 50, got %d", baseTransport.MaxIdleConnsPerHost)
	}
}
