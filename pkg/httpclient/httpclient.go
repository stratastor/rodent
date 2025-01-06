/*
 * Copyright 2024 Raamsri Kumar <raam@tinkershack.in> and The StrataSTOR Authors 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */package httpclient

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stratastor/rodent/internal/constants"
)

const (
	defaultTimeout         = 10 * time.Second
	defaultRetryCount      = 3
	defaultRetryWaitTime   = 2 * time.Second
	defaultRetryMaxWait    = 10 * time.Second
	defaultMaxIdleConns    = 100
	defaultIdleConnTimeout = 90 * time.Second
	defaultUserAgent       = "Rodent-Agent"
)

// Client wraps resty.Client with additional functionality
type Client struct {
	*resty.Client
	config ClientConfig
}

// ClientConfig holds configuration values for the HTTP client
type ClientConfig struct {
	// Basic settings
	BaseURL          string
	Timeout          time.Duration
	RetryCount       int
	RetryWaitTime    time.Duration
	RetryMaxWaitTime time.Duration
	RetryConditions  []resty.RetryConditionFunc
	UserAgent        string

	// Security settings
	TLSConfig      *tls.Config
	AllowInsecure  bool
	ClientCertPath string
	ClientKeyPath  string
	CACertPath     string

	// Request settings
	Headers      map[string]string
	QueryParams  map[string]string
	Cookies      []*http.Cookie
	DisableWarn  bool
	AllowGetBody bool

	// Transport settings
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	DisableCompression  bool
	DisableKeepAlives   bool

	// Authentication
	BasicAuth struct {
		Username string
		Password string
	}
	BearerToken string

	// Debug settings
	Debug          bool
	DebugBodyLimit int64
	EnableTrace    bool
}

// NewClientConfig returns a ClientConfig with sensible defaults
func NewClientConfig() ClientConfig {
	return ClientConfig{
		BaseURL:             "",
		RetryConditions:     nil,
		TLSConfig:           nil,
		AllowInsecure:       false,
		ClientCertPath:      "",
		ClientKeyPath:       "",
		CACertPath:          "",
		Headers:             make(map[string]string),
		QueryParams:         make(map[string]string),
		Cookies:             nil,
		DisableWarn:         false,
		AllowGetBody:        false,
		MaxIdleConns:        defaultMaxIdleConns,
		MaxIdleConnsPerHost: 0,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     defaultIdleConnTimeout,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		BasicAuth: struct {
			Username string
			Password string
		}{},
		BearerToken:      "",
		Debug:            false,
		DebugBodyLimit:   0,
		EnableTrace:      false,
		Timeout:          defaultTimeout,
		RetryCount:       defaultRetryCount,
		RetryWaitTime:    defaultRetryWaitTime,
		RetryMaxWaitTime: defaultRetryMaxWait,
		UserAgent:        defaultUserAgent + "/" + constants.RodentVersion,
	}
}

// NewClient creates a new Resty client with provided configuration
func NewClient(config ClientConfig) *Client {
	restyClient := resty.New()
	client := &Client{
		Client: restyClient,
		config: config,
	}

	// Apply client configuration
	client.applyConfig()

	return client
}

// applyConfig applies the client configuration
func (c *Client) applyConfig() {

	if c.config.Timeout > 0 {
		c.Client.SetTimeout(c.config.Timeout)
	}
	if c.config.RetryCount > 0 {
		c.Client.SetRetryCount(c.config.RetryCount)
	}
	if c.config.RetryWaitTime > 0 {
		c.Client.SetRetryWaitTime(c.config.RetryWaitTime)
	}
	if c.config.RetryMaxWaitTime > 0 {
		c.Client.SetRetryMaxWaitTime(c.config.RetryMaxWaitTime)
	}
	if c.config.UserAgent != "" {
		c.Client.SetHeader("User-Agent", c.config.UserAgent)
	}
	if c.config.BaseURL != "" {
		c.Client.SetBaseURL(c.config.BaseURL)
	}
	if c.config.Headers != nil {
		c.Client.SetHeaders(c.config.Headers)
	}
	if c.config.BasicAuth.Username != "" && c.config.BasicAuth.Password != "" {
		c.Client.SetBasicAuth(c.config.BasicAuth.Username, c.config.BasicAuth.Password)
	}
	if c.config.BearerToken != "" {
		c.Client.SetAuthToken(c.config.BearerToken)
	}
	if c.config.Debug == true {
		c.Client.SetDebug(true)
		if c.config.DebugBodyLimit > 0 {
			c.Client.SetDebugBodyLimit(c.config.DebugBodyLimit)
		}
	} else {
		c.Client.SetDebug(false)
		// Suppress Resty logs by setting a no-op logger
		c.Client.SetLogger(NoOpLogger{})
	}
	if c.config.EnableTrace == true {
		c.Client.EnableTrace()
	}
	if len(c.config.RetryConditions) > 0 {
		for _, condition := range c.config.RetryConditions {
			c.Client.AddRetryCondition(condition)
		}
	}

	// Configure transport
	transport := &http.Transport{
		MaxIdleConns:        c.config.MaxIdleConns,
		MaxIdleConnsPerHost: c.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     c.config.MaxConnsPerHost,
		IdleConnTimeout:     c.config.IdleConnTimeout,
		DisableCompression:  c.config.DisableCompression,
		DisableKeepAlives:   c.config.DisableKeepAlives,
	}

	// Configure TLS
	if c.config.TLSConfig != nil {
		transport.TLSClientConfig = c.config.TLSConfig
	} else if c.config.AllowInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	c.Client.SetTransport(transport)
}

// NoOpLogger suppresses all logs
type NoOpLogger struct{}

// Printf is a no-op implementation of the Printf method
func (l NoOpLogger) Printf(format string, v ...interface{}) {
	// Do nothing
}

// Debugf is a no-op implementation of the Debugf method
func (l NoOpLogger) Debugf(format string, v ...interface{}) {
	// Do nothing
}

// Warnf is a no-op implementation of the Warnf method
func (l NoOpLogger) Warnf(format string, v ...interface{}) {
	// Do nothing
}

// Errorf is a no-op implementation of the Errorf method
func (l NoOpLogger) Errorf(format string, v ...interface{}) {
	// Do nothing
}

// ValidateConfig checks if the configuration is valid
func ValidateConfig(config ClientConfig) error {
	// TODO: Implement validation logic
	return nil
}

// RequestConfig holds request-level parameters
type RequestConfig struct {
	Path        string
	Headers     map[string]string
	QueryParams map[string]string
	FormData    map[string]string
	Body        interface{}
	Result      interface{}
	Error       interface{}
	Context     context.Context
}

// Request wraps resty.Request with additional functionality
type Request struct {
	client  *Client
	request *resty.Request
	config  RequestConfig
}

// NewRequest creates a new request with given configuration
func (c *Client) NewRequest(cfg RequestConfig) *Request {
	req := &Request{
		client:  c,
		request: c.R(),
		config:  cfg,
	}

	// Apply request configuration
	if cfg.Headers != nil {
		req.request.SetHeaders(cfg.Headers)
	}
	if cfg.QueryParams != nil {
		req.request.SetQueryParams(cfg.QueryParams)
	}
	if cfg.FormData != nil {
		req.request.SetFormData(cfg.FormData)
	}
	if cfg.Body != nil {
		req.request.SetBody(cfg.Body)
	}
	if cfg.Result != nil {
		req.request.SetResult(cfg.Result)
	}
	if cfg.Error != nil {
		req.request.SetError(cfg.Error)
	}
	if cfg.Context != nil {
		req.request.SetContext(cfg.Context)
	}

	return req
}

// Execute performs the HTTP request with the specified method
func (r *Request) Execute(method string) (*resty.Response, error) {
	return r.request.Execute(method, r.config.Path)
}

// Convenience methods for common HTTP methods
func (r *Request) Get() (*resty.Response, error) {
	return r.Execute(http.MethodGet)
}

func (r *Request) Post() (*resty.Response, error) {
	return r.Execute(http.MethodPost)
}

func (r *Request) Put() (*resty.Response, error) {
	return r.Execute(http.MethodPut)
}

func (r *Request) Delete() (*resty.Response, error) {
	return r.Execute(http.MethodDelete)
}
