package shttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type APIClientCfg struct {
	Client  *http.Client `json:"-"`
	BaseURI string       `json:"baseURI"`
}

type APIClient struct {
	Client *http.Client

	Cookie *http.Cookie

	baseURI *url.URL
}

func NewAPIClient(cfg APIClientCfg) (*APIClient, error) {
	baseURI, err := url.Parse(cfg.BaseURI)
	if err != nil {
		return nil, fmt.Errorf("invalid base uri: %w", err)
	}

	c := APIClient{
		Client: cfg.Client,

		baseURI: baseURI,
	}

	return &c, nil
}

func (c *APIClient) SendRequest(method, uriRefString string, reqBody, resBody interface{}) (*http.Response, error) {
	uriRef, err := url.Parse(uriRefString)
	if err != nil {
		return nil, fmt.Errorf("invalid uri reference: %w", err)
	}

	uri := c.baseURI.ResolveReference(uriRef)

	var reqBodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("cannot encode request body: %w", err)
		}

		reqBodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, uri.String(), reqBodyReader)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	if c.Cookie != nil {
		req.Header.Add("Cookie", c.Cookie.String())
	}

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot send request: %w", err)
	}
	defer res.Body.Close()

	status := res.StatusCode

	resBodyData, err := io.ReadAll(res.Body)
	if err != nil {
		return res, fmt.Errorf("cannot read response body: %w", err)
	}

	if status < 200 || status >= 400 {
		var baseError error

		var apiError JSONError
		if err := json.Unmarshal(resBodyData, &apiError); err == nil {
			baseError = &apiError
		} else {
			baseError = errors.New(string(resBodyData))
		}

		return res, fmt.Errorf("request failed with status %d: %w",
			res, baseError)
	}

	if resBody != nil {
		if err := json.Unmarshal(resBodyData, resBody); err != nil {
			return res, fmt.Errorf("cannot decode response body: %w", err)
		}
	}

	return res, nil
}
