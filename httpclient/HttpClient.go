// please refer to https://github.com/gojek/heimdall?tab=readme-ov-file#making-a-simple-get-request

package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gojek/heimdall"
	"github.com/gojek/heimdall/v7/httpclient"
	"github.com/gojek/heimdall/v7/hystrix"
)

type Client interface {
	Get(path string, queryParams map[string]string, response any) error
	Post(path string, data any, response any) error
	Put(path string, data any, response any) error
	Delete(path string) error
}

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
}

type circuitClient struct {
	baseURL string
	client  *hystrix.Client
}

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

func NewHystixClient(baseURL string, config ClientConfig, fallbackFn func(error) error) *circuitClient {

	config.applyDefaults()

	bo := heimdall.NewExponentialBackoff(config.BackoffInitial, config.BackoffMax, 2.0, config.BackoffMax)

	httpClient := httpclient.NewClient(
		httpclient.WithHTTPTimeout(config.Timeout),
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

// In some cases we need to pass the http.Client with all the rery, circuit break logic.
func NewHTTPClient(baseURL string, config ClientConfig, fallbackFn func(error) error) *http.Client {

	hystrixClient := NewHystixClient(baseURL, config, fallbackFn)

	return &http.Client{
		Transport: &hystrixRoundTripper{client: hystrixClient.client},
	}
}

type hystrixRoundTripper struct {
	client *hystrix.Client
}

func (rt *hystrixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.client.Do(req)
}

func (c *circuitClient) Get(path string, queryParams map[string]string, response any) error {
	fullPath := path
	if len(queryParams) > 0 {
		u := url.URL{Path: path}
		q := u.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		fullPath = u.String()
	}
	return c.doRequest(http.MethodGet, fullPath, nil, response)
}

func (c *circuitClient) Post(path string, data any, response any) error {
	return c.doRequest(http.MethodPost, path, data, response)
}

func (c *circuitClient) Put(path string, data any, response any) error {
	return c.doRequest(http.MethodPut, path, data, response)
}

func (c *circuitClient) Delete(path string) error {
	return c.doRequest(http.MethodDelete, path, nil, nil)
}

func (c *circuitClient) doRequest(method, path string, requestBody any, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if responseBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}
