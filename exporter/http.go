package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/mcorbin/cabourotte/healthcheck"
	"github.com/mcorbin/cabourotte/tls"
)

// HTTPConfiguration The configuration for the HTTP exporter.
type HTTPConfiguration struct {
	Name     string
	Host     string
	Path     string
	Port     uint32
	Protocol healthcheck.Protocol
	Key      string `json:"key,omitempty"`
	Cert     string `json:"cert,omitempty"`
	Cacert   string `json:"cacert,omitempty"`
	Insecure bool
}

// HTTPExporter the http exporter struct
type HTTPExporter struct {
	Started bool
	Logger  *zap.Logger
	URL     string
	Config  *HTTPConfiguration
	Client  *http.Client
}

// UnmarshalYAML parses the configuration of the http component from YAML.
func (c *HTTPConfiguration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawConfiguration HTTPConfiguration
	raw := rawConfiguration{}
	if err := unmarshal(&raw); err != nil {
		return errors.Wrap(err, "Unable to read HTTP exporter configuration")
	}
	if raw.Host == "" {
		return errors.New("Invalid host for the HTTP exporter configuration")
	}
	if raw.Name == "" {
		return errors.New("Invalid name for the HTTP exporter configuration")
	}
	if raw.Port == 0 {
		return errors.New("Invalid port for the HTTP server")
	}
	if !((raw.Key != "" && raw.Cert != "") ||
		(raw.Key == "" && raw.Cert == "")) {
		return errors.New("Invalid certificates")
	}
	*c = HTTPConfiguration(raw)
	return nil
}

// NewHTTPExporter creates a new HTTP exporter
func NewHTTPExporter(logger *zap.Logger, config *HTTPConfiguration) (*HTTPExporter, error) {
	protocol := "http"
	tlsConfig, err := tls.GetTLSConfig(config.Key, config.Cert, config.Cacert, config.Insecure)
	if err != nil {
		return nil, err
	}
	if config.Protocol == healthcheck.HTTPS {
		protocol = "https"
	}
	url := fmt.Sprintf(
		"%s://%s%s",
		protocol,
		net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)),
		config.Path)
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	exporter := HTTPExporter{
		Logger: logger,
		Config: config,
		URL:    url,
		Client: &http.Client{
			Transport: transport,
			Timeout:   time.Second * 3,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	return &exporter, nil
}

// IsStarted returns the exporter status
func (c *HTTPExporter) IsStarted() bool {
	return c.Started
}

// Start starts the HTTP exporter component
func (c *HTTPExporter) Start() error {
	// nothing to do
	c.Logger.Info(fmt.Sprintf("Starting the HTTP healthcheck exporter on %s:%d", c.Config.Host, c.Config.Port))
	c.Started = true
	return nil
}

// Reconnect reconnects the HTTP exporter component
func (c *HTTPExporter) Reconnect() error {
	// nothing to do
	c.Started = true
	return nil
}

// Stop stops the HTTP exporter component
func (c *HTTPExporter) Stop() error {
	c.Logger.Info(fmt.Sprintf("Stopping the http exporter %s", c.Config.Name))
	c.Started = false
	return nil
}

// Name returns the name of the exporter
func (c *HTTPExporter) Name() string {
	return c.Config.Name
}

// GetConfig returns the config of the exporter
func (c *HTTPExporter) GetConfig() interface{} {
	return c.Config
}

// Push pushes events to the HTTP destination
func (c *HTTPExporter) Push(result *healthcheck.Result) error {
	var jsonBytes []byte
	payload := []*healthcheck.Result{result}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrapf(err, "Fail to convert result to json:\n%v", result)
	}
	req, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return errors.Wrapf(err, "HTTP exporter: fail to create request for %s", c.URL)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "HTTP exporter: fail to send healthchecks to %s", c.URL)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP exporter: request failed, status %d", resp.StatusCode)
	}
	return nil
}
