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

type APIClientErrorHandler func(*http.Response, []byte) error

type APIClientCfg struct {
	Client      *http.Client          `json:"-"`
	HandleError APIClientErrorHandler `json:"-"`
	BaseURI     string                `json:"base_uri"`
}

type APIClient struct {
	Cfg    APIClientCfg
	Client *http.Client

	BasicUsername string
	BasicPassword string
	BearerToken   string
	Cookie        *http.Cookie

	baseURI *url.URL
}

func NewAPIClient(cfg APIClientCfg) (*APIClient, error) {
	if cfg.HandleError == nil {
		cfg.HandleError = HandleAPIClientError
	}

	baseURI, err := url.Parse(cfg.BaseURI)
	if err != nil {
		return nil, fmt.Errorf("invalid base URI: %w", err)
	}

	c := APIClient{
		Cfg:     cfg,
		baseURI: baseURI,
	}

	return &c, nil
}

func (c *APIClient) SendRequest(method, uriRefString string, reqBody, resBody any) (*http.Response, error) {
	return c.SendRequestWithHeader(method, uriRefString, nil, reqBody, resBody)
}

func (c *APIClient) SendRequestWithHeader(method, uriRefString string, header http.Header, reqBody, resBody any) (*http.Response, error) {
	uriRef, err := url.Parse(uriRefString)
	if err != nil {
		return nil, fmt.Errorf("invalid URI reference: %w", err)
	}

	uri := c.baseURI.ResolveReference(uriRef)

	var reqBodyReader io.Reader
	if reqBody != nil {
		if r, ok := reqBody.(io.Reader); ok {
			reqBodyReader = r
		} else {
			data, err := json.Marshal(reqBody)
			if err != nil {
				return nil, fmt.Errorf("cannot encode request body: %w", err)
			}

			reqBodyReader = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequest(method, uri.String(), reqBodyReader)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	if c.BasicUsername != "" {
		req.SetBasicAuth(c.BasicUsername, c.BasicPassword)
	}

	if c.BearerToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.BearerToken)
	}

	if c.Cookie != nil {
		req.Header.Add("Cookie", c.Cookie.String())
	}

	res, err := c.Cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot send request: %w", err)
	}
	defer res.Body.Close()

	resBodyData, err := io.ReadAll(res.Body)
	if err != nil {
		return res, fmt.Errorf("cannot read response body: %w", err)
	}

	// We want the caller to be able to decode the response body again if
	// necessary.
	res.Body = io.NopCloser(bytes.NewBuffer(resBodyData))

	if err := c.Cfg.HandleError(res, resBodyData); err != nil {
		return res, err
	}

	if resBody != nil {
		if err := json.Unmarshal(resBodyData, resBody); err != nil {
			return res, fmt.Errorf("cannot decode response body: %w", err)
		}
	}

	return res, nil
}

func HandleAPIClientError(res *http.Response, body []byte) error {
	if status := res.StatusCode; status >= 200 && status < 400 {
		return nil
	}

	var baseError error

	var apiError JSONError
	if err := json.Unmarshal(body, &apiError); err == nil {
		baseError = &apiError
	} else {
		baseError = errors.New(string(body))
	}

	return fmt.Errorf("request failed with status %d: %w",
		res.StatusCode, baseError)
}
