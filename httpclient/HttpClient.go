// Package httpclient provides an HTTP client with retry, circuit breaker, and mTLS/TLS support.
// Built on top of gojek/heimdall with Hystrix circuit breaker and exponential backoff.
//
// Reference: https://github.com/gojek/heimdall
package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gojek/heimdall"
	"github.com/gojek/heimdall/v7/httpclient"
	"github.com/gojek/heimdall/v7/hystrix"
	"golang.org/x/crypto/pkcs12"
)

// ============================================================================
// Interfaces & Configurations
// ============================================================================

// Client defines a high-level HTTP client interface with common methods.
type Client interface {
	Get(path string, queryParams map[string]string, response any) error
	Post(path string, data any, response any) error
	Put(path string, data any, response any) error
	Delete(path string) error
	WithOverrideBaseURL(url string) Client
	DoRequest(method, path string, queryParams map[string]string, requestBody any, responseBody any, headers map[string]string) error
	DownloadToFile(method, path string, queryParams map[string]string, body any, outputDir string, headers []string) (*DownloadFileResponse, error)
}

// ClientConfig defines configuration for retries, backoff, and circuit breaker.
type ClientConfig struct {
	Timeout                time.Duration
	RetryCount             int
	BackoffInitial         time.Duration
	BackoffMax             time.Duration
	CircuitBreakerCommand  string
	CircuitBreakerTimeout  time.Duration
	MaxConcurrentRequests  int
	ErrorPercentThreshold  int
	SleepWindow            int
	RequestVolumeThreshold int
	TLSClientConfig        TLSClientConfig
}

// TLSClientConfig supports TLS and mTLS configurations.
type TLSClientConfig struct {
	// TLS/server verification
	SkipTLSVerify bool
	CACertPaths   []string // Custom CA roots for server verification

	// mTLS: client authentication
	ClientCertPath string
	ClientKeyPath  string

	// In-memory PEM
	ClientCertPEM string
	ClientKeyPEM  string

	// PKCS#12 (.p12 / .pfx)
	ClientP12Path     string
	ClientP12Value    string
	ClientP12Password string
}

// ============================================================================
// Implementation
// ============================================================================

type circuitClient struct {
	baseURL string
	client  *hystrix.Client
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() ClientConfig {
	return ClientConfig{
		Timeout:                30 * time.Second,
		RetryCount:             3,
		BackoffInitial:         100 * time.Millisecond,
		BackoffMax:             5 * time.Second,
		CircuitBreakerCommand:  "http-client",
		CircuitBreakerTimeout:  10 * time.Second,
		MaxConcurrentRequests:  100,
		ErrorPercentThreshold:  25,
		SleepWindow:            10,
		RequestVolumeThreshold: 10,
	}
}

func (c *ClientConfig) applyDefaults() {
	defaults := DefaultConfig()
	if c.Timeout == 0 {
		c.Timeout = defaults.Timeout
	}
	if c.RetryCount == 0 {
		c.RetryCount = defaults.RetryCount
	}
	if c.BackoffInitial == 0 {
		c.BackoffInitial = defaults.BackoffInitial
	}
	if c.BackoffMax == 0 {
		c.BackoffMax = defaults.BackoffMax
	}
	if c.CircuitBreakerCommand == "" {
		c.CircuitBreakerCommand = defaults.CircuitBreakerCommand
	}
	if c.CircuitBreakerTimeout == 0 {
		c.CircuitBreakerTimeout = defaults.CircuitBreakerTimeout
	}
	if c.MaxConcurrentRequests == 0 {
		c.MaxConcurrentRequests = defaults.MaxConcurrentRequests
	}
	if c.ErrorPercentThreshold == 0 {
		c.ErrorPercentThreshold = defaults.ErrorPercentThreshold
	}
	if c.SleepWindow == 0 {
		c.SleepWindow = defaults.SleepWindow
	}
	if c.RequestVolumeThreshold == 0 {
		c.RequestVolumeThreshold = defaults.RequestVolumeThreshold
	}
}

// ============================================================================
// Client Construction
// ============================================================================

// NewHystixClient creates a Heimdall Hystrix client with retries, backoff, and optional TLS/mTLS.
func NewHystixClient(baseURL string, config ClientConfig, fallbackFn func(error) error) *circuitClient {
	config.applyDefaults()

	bo := heimdall.NewExponentialBackoff(config.BackoffInitial, config.BackoffMax, 2.0, config.BackoffMax)
	transport, err := createCustomTransport(config.TLSClientConfig)
	if err != nil {
		panic(fmt.Errorf("failed to create custom TLS transport: %w", err))
	}

	httpClient := httpclient.NewClient(
		httpclient.WithHTTPClient(&http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		}),
		httpclient.WithRetryCount(config.RetryCount),
		httpclient.WithRetrier(heimdall.NewRetrier(bo)),
	)

	hystrixClient := hystrix.NewClient(
		hystrix.WithHTTPClient(httpClient),
		hystrix.WithCommandName(config.CircuitBreakerCommand),
		hystrix.WithHystrixTimeout(config.CircuitBreakerTimeout),
		hystrix.WithMaxConcurrentRequests(config.MaxConcurrentRequests),
		hystrix.WithErrorPercentThreshold(config.ErrorPercentThreshold),
		hystrix.WithSleepWindow(config.SleepWindow),
		hystrix.WithRequestVolumeThreshold(config.RequestVolumeThreshold),
		hystrix.WithFallbackFunc(fallbackFn),
	)

	return &circuitClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  hystrixClient,
	}
}

// NewHTTPClient wraps a Hystrix client in a standard *http.Client.
func NewHTTPClient(baseURL string, config ClientConfig, fallbackFn func(error) error) *http.Client {
	hc := NewHystixClient(baseURL, config, fallbackFn)
	return &http.Client{Transport: &hystrixRoundTripper{client: hc.client}}
}

type hystrixRoundTripper struct{ client *hystrix.Client }

func (rt *hystrixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Propagate trace_id from context to request header for distributed tracing
	propagateTraceID(req)
	return rt.client.Do(req)
}

// ============================================================================
// Tracing HTTP Client
// ============================================================================

// TracingRoundTripper wraps an http.RoundTripper and propagates trace_id from context
type TracingRoundTripper struct {
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper and adds trace_id header propagation
func (t *TracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	propagateTraceID(req)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewTracingHTTPClient creates a simple *http.Client that automatically propagates trace_id
// from request context to X-Trace-ID header. Use this for simple HTTP clients without
// circuit breaker or retry logic.
func NewTracingHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &TracingRoundTripper{
			Base: http.DefaultTransport,
		},
	}
}

// propagateTraceID extracts trace_id from request context and sets it as X-Trace-ID header
func propagateTraceID(req *http.Request) {
	if req == nil || req.Context() == nil {
		return
	}
	if traceID := api.GetTraceIDFromContext(req.Context()); traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
	}
}

// ============================================================================
// Request Methods
// ============================================================================

func (c *circuitClient) Get(path string, queryParams map[string]string, response any) error {
	return c.DoRequest(http.MethodGet, path, queryParams, nil, response, nil)
}

func (c *circuitClient) Post(path string, data any, response any) error {
	return c.DoRequest(http.MethodPost, path, nil, data, response, nil)
}

func (c *circuitClient) Put(path string, data any, response any) error {
	return c.DoRequest(http.MethodPut, path, nil, data, response, nil)
}

func (c *circuitClient) Delete(path string) error {
	return c.DoRequest(http.MethodDelete, path, nil, nil, nil, nil)
}

func (c *circuitClient) WithOverrideBaseURL(baseURL string) Client {
	return &circuitClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  c.client,
	}
}

// Core unified request method.
func (c *circuitClient) DoRequest(method, path string, queryParams map[string]string, requestBody any, responseBody any, headers map[string]string) error {
	var body io.Reader
	switch v := requestBody.(type) {
	case nil:
	case io.Reader:
		body = v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewBuffer(data)
	}

	finalURL := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		finalURL = c.BuildURL(path)
	}
	finalURL = InjectQueryParams(finalURL, queryParams)

	req, err := http.NewRequest(method, finalURL, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	if responseBody != nil {
		return json.NewDecoder(resp.Body).Decode(responseBody)
	}
	return nil
}

// BuildURL constructs a full URL safely.
func (c *circuitClient) BuildURL(paths ...string) string {
	base := strings.TrimRight(c.baseURL, "/")
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		if trimmed := strings.Trim(p, "/"); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return base + "/" + strings.Join(parts, "/")
}

func InjectQueryParams(rawURL string, queryParams map[string]string) string {
	if len(queryParams) == 0 {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for k, v := range queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ============================================================================
// File Download Helper
// ============================================================================

type DownloadFileResponse struct {
	Filename          string
	AdditionalDetails map[string]string
}

func (c *circuitClient) DownloadToFile(method, path string, queryParams map[string]string, body any, outputDir string, headerKeys []string) (*DownloadFileResponse, error) {
	finalURL := InjectQueryParams(c.BuildURL(path), queryParams)

	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		requestBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, finalURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	filename := fmt.Sprintf("download-%d.bin", time.Now().Unix())
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if parts := strings.Split(cd, "filename="); len(parts) == 2 {
			filename = strings.Trim(parts[1], `"`)
		}
	}
	outputPath := filepath.Join(outputDir, filename)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, resp.Body); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	headers := map[string]string{}
	for _, key := range headerKeys {
		if val := resp.Header.Get(key); val != "" {
			headers[key] = val
		}
	}

	return &DownloadFileResponse{Filename: outputPath, AdditionalDetails: headers}, nil
}

// ============================================================================
// TLS / mTLS Support
// ============================================================================

func createCustomTransport(cfg TLSClientConfig) (*http.Transport, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.SkipTLSVerify, Renegotiation: tls.RenegotiateOnceAsClient}

	if len(cfg.CACertPaths) > 0 {
		certPool := x509.NewCertPool()
		for _, path := range cfg.CACertPaths {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read CA cert: %w", err)
			}
			if !certPool.AppendCertsFromPEM(data) {
				return nil, fmt.Errorf("append CA cert failed: %s", path)
			}
		}
		tlsConfig.RootCAs = certPool
	}

	if cert, ok, err := loadClientCertificate(cfg); err != nil {
		return nil, err
	} else if ok {
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}

// Tries all supported mTLS sources in priority order.
func loadClientCertificate(cfg TLSClientConfig) (tls.Certificate, bool, error) {
	// PEM file paths
	if cfg.ClientCertPath != "" && cfg.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("load PEM certs: %w", err)
		}
		return cert, true, nil
	}

	// In-memory PEM
	if cfg.ClientCertPEM != "" && cfg.ClientKeyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(cfg.ClientCertPEM), []byte(cfg.ClientKeyPEM))
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("parse in-memory PEM: %w", err)
		}
		return cert, true, nil
	}

	// PKCS#12 file
	if cfg.ClientP12Path != "" && cfg.ClientP12Password != "" {
		data, err := os.ReadFile(cfg.ClientP12Path)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("read p12 file: %w", err)
		}
		cfg.ClientP12Value = string(data)
	}

	// PKCS#12 in-memory
	if cfg.ClientP12Value != "" && cfg.ClientP12Password != "" {
		blocks, err := pkcs12.ToPEM([]byte(cfg.ClientP12Value), cfg.ClientP12Password)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("decode p12: %w", err)
		}
		var pemData []byte
		for _, b := range blocks {
			pemData = append(pemData, pem.EncodeToMemory(b)...)
		}
		cert, err := tls.X509KeyPair(pemData, pemData)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf("parse X509 from p12: %w", err)
		}
		return cert, true, nil
	}

	return tls.Certificate{}, false, nil
}
