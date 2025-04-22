package influx

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

type ClientCfg struct {
	Log        *log.Logger  `json:"-"`
	HTTPClient *http.Client `json:"-"`
	Hostname   string       `json:"-"`

	URI         string            `json:"uri"`
	Bucket      string            `json:"bucket"`
	Org         string            `json:"org,omitempty"`
	Token       string            `json:"token,omitempty"`
	BatchSize   int               `json:"batch_size,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	LogRequests bool              `json:"log_requests,omitempty"`
}

type Client struct {
	Cfg        ClientCfg
	Log        *log.Logger
	HTTPClient *http.Client

	uri  *url.URL
	tags map[string]string

	points     Points
	pointMutex sync.Mutex

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func (cfg *ClientCfg) ValidateJSON(v *ejson.Validator) {
	if cfg.URI != "" {
		v.CheckStringURI("uri", cfg.URI)
	}

	v.CheckStringNotEmpty("bucket", cfg.Bucket)

	v.Push("tags")
	for name, value := range cfg.Tags {
		v.CheckStringNotEmpty(name, value)
	}
	v.Pop()
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("influx")
	}

	if cfg.HTTPClient == nil {
		return nil, fmt.Errorf("missing HTTP client")
	}

	if cfg.URI == "" {
		cfg.URI = "http://localhost:8086"
	}
	uri, err := url.Parse(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10_000
	}

	tags := make(map[string]string)
	if cfg.Hostname != "" {
		tags["host"] = cfg.Hostname
	}
	for name, value := range cfg.Tags {
		tags[name] = value
	}

	c := &Client{
		Cfg:        cfg,
		Log:        cfg.Log,
		HTTPClient: cfg.HTTPClient,

		uri:  uri,
		tags: tags,

		stopChan: make(chan struct{}),
	}

	return c, nil
}

func (c *Client) Start() {
	c.wg.Add(1)
	go c.main()

	c.wg.Add(1)
	go c.goProbeMain()
}

func (c *Client) Stop() {
	close(c.stopChan)
	c.wg.Wait()

	c.HTTPClient.CloseIdleConnections()
}

func (c *Client) Terminate() {
}

func (c *Client) main() {
	defer c.wg.Done()

	timer := time.NewTicker(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-c.stopChan:
			c.flush()
			return

		case <-timer.C:
			c.flush()
		}
	}
}

func (c *Client) EnqueuePoint(p *Point) {
	c.EnqueuePoints(Points{p})
}

func (c *Client) EnqueuePoints(points Points) {
	select {
	case <-c.stopChan:
		return
	default:
	}

	c.finalizePoints(points)

	c.pointMutex.Lock()
	c.points = append(c.points, points...)
	flush := len(c.points) >= c.Cfg.BatchSize
	c.pointMutex.Unlock()

	if flush {
		c.flush()
	}
}

func (c *Client) SendPoints(points Points) error {
	// Most of the time, it is more important to avoid blocking the service than
	// to guarantee metric delivery. In some specific situations, the job of the
	// service is to produce metrics, and it is absolutely fine to wait until we
	// have made sure that they have been successfully sent to Influx. In these
	// situations, we want the ability to send points directly instead of
	// queuing them.

	c.finalizePoints(points)
	return c.sendPoints(points)
}

func (c *Client) finalizePoints(points Points) {
	// Point finalization is idempotent, we rely on this property

	for _, p := range points {
		// We do not have to protect access to the client tag map because it is
		// read-only after initialization.
		if p.Tags == nil && len(c.tags) > 0 {
			p.Tags = Tags{}
		}

		// Tags in the point override tags in the client
		for name, value := range c.tags {
			if _, found := p.Tags[name]; !found {
				c.tags[name] = value
			}
		}
	}
}

func (c *Client) flush() {
	// Take points out of the queue; this is necessary to allow multiple
	// concurrent calls to flush().

	c.pointMutex.Lock()
	points := c.points
	c.points = nil
	c.pointMutex.Unlock()

	if len(points) == 0 {
		return
	}

	go func() {
		if err := c.sendPoints(points); err != nil {
			c.Log.Error("cannot send points: %v", err)
		}

		// If we cannot send the points, we put them back in the queue. Ordering
		// does not really matter, so we add them at the end and not at the
		// beginning because it avoids an unnecessary copy.
		//
		// Since point finalization is idempotent, it does not matter that it
		// will be done again next time we flush.

		c.pointMutex.Lock()
		c.points = append(c.points, points...)
		c.pointMutex.Unlock()
	}()
}

func (c *Client) sendPoints(points Points) error {
	// Remember that the function can be called from another goroutine through
	// SendPoints.

	uri := *c.uri
	uri.Path = path.Join(uri.Path, "/api/v2/write")

	query := url.Values{}
	query.Set("bucket", c.Cfg.Bucket)
	if c.Cfg.Org != "" {
		query.Set("org", c.Cfg.Org)
	}

	uri.RawQuery = query.Encode()

	var buf bytes.Buffer
	if err := EncodePoints(points, &buf); err != nil {
		return fmt.Errorf("cannot encode points: %w", err)
	}

	req, err := http.NewRequest("POST", uri.String(), &buf)
	if err != nil {
		return fmt.Errorf("cannot create request: %w", err)
	}

	if c.Cfg.Token != "" {
		req.Header.Add("Authorization", "Token "+c.Cfg.Token)
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot send request: %w", err)
	}
	defer res.Body.Close()

	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		bodyData, err := ioutil.ReadAll(res.Body)
		if err != nil {
			c.Log.Error("cannot read response body: %v", err)
		}

		bodyString := ""
		if bodyData != nil {
			// Influx can send incredibly long error messages, sometimes
			// including the entire payload received. This is very annoying,
			// but even if it was to be patched, we would still have to
			// support old versions.
			if len(bodyData) > 200 {
				bodyData = append(bodyData[:200], []byte(" [truncated]")...)
			}

			bodyString = " (" + string(bodyData) + ")"
		}

		return fmt.Errorf("request failed with status %d%s",
			res.StatusCode, bodyString)
	}

	return nil
}
