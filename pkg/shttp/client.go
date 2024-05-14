package shttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/galdor/go-ejson"
	"github.com/galdor/go-log"
	"github.com/galdor/go-service/pkg/utils"
)

const (
	DefaultClientConnectionTimeout = 30 // seconds
	DefaultClientRequestTimeout    = 30 // seconds
)

type ClientCfg struct {
	Log *log.Logger `json:"-"`

	ConnectionTimeout *int `json:"connection_timeout"` // seconds
	RequestTimeout    *int `json:"request_timeout"`    // seconds

	LogRequests         bool `json:"log_requests"`
	DisableRedirections bool `json:"disable_redirections"`

	TLS *TLSClientCfg `json:"tls"`

	Header http.Header `json:"-"`
}

type TLSClientCfg struct {
	CACertificates []string `json:"ca_certificates"`
}

type Client struct {
	Cfg ClientCfg
	Log *log.Logger

	Client *http.Client

	tlsCfg *tls.Config
}

func (cfg *ClientCfg) ValidateJSON(v *ejson.Validator) {
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.ConnectionTimeout == nil {
		cfg.ConnectionTimeout = utils.Ref(DefaultClientConnectionTimeout)
	}

	if cfg.RequestTimeout == nil {
		cfg.RequestTimeout = utils.Ref(DefaultClientRequestTimeout)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,

		DialContext: (&net.Dialer{
			Timeout:   time.Duration(*cfg.ConnectionTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		MaxIdleConns: 100,

		IdleConnTimeout:       60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	tlsCfg := &tls.Config{}

	if cfg.TLS != nil {
		caCertificatePool, err := LoadCertificates(cfg.TLS.CACertificates)
		if err != nil {
			return nil, err
		}

		tlsCfg.RootCAs = caCertificatePool
	}

	client := &http.Client{
		Timeout:   time.Duration(*cfg.RequestTimeout) * time.Second,
		Transport: NewRoundTripper(transport, &cfg),
	}

	if cfg.DisableRedirections {
		client.CheckRedirect =
			func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
	}

	c := &Client{
		Cfg: cfg,
		Log: cfg.Log,

		Client: client,

		tlsCfg: tlsCfg,
	}

	transport.DialTLSContext = c.DialTLSContext

	return c, nil
}

func (c *Client) CloseConnections() {
	c.Client.CloseIdleConnections()
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.Client.Do(req)
}

func (c *Client) DialTLSContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		Config: c.tlsCfg,
	}

	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func LoadCertificates(certificates []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	for _, certificate := range certificates {
		data, err := os.ReadFile(certificate)
		if err != nil {
			return nil, fmt.Errorf("cannot read %q: %w", certificate, err)
		}

		if pool.AppendCertsFromPEM(data) == false {
			return nil, fmt.Errorf("cannot load certificates from %q",
				certificate)
		}
	}

	return pool, nil
}
