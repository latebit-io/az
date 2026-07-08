package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// client is a thin JSON helper over the az HTTP API — extractable later into
// an az-guard client library.
type client struct {
	baseURI string
	http    *http.Client
	apiKey  string
}

func newClient(baseURI string) *client {
	return &client{baseURI: baseURI, http: &http.Client{}}
}

// do sends a JSON request and decodes the response into out when non-nil.
// It returns the response status code.
func (c *client) do(method, path string, body any, out any) (int, error) {
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}

	request, err := http.NewRequest(method, c.baseURI+path, reader)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("X-AZ-API-KEY", c.apiKey)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if out != nil && response.StatusCode < 300 {
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			return response.StatusCode, fmt.Errorf("decode response for %s %s: %w", method, path, err)
		}
	}
	return response.StatusCode, nil
}

func (c *client) post(path string, body any, out any) (int, error) {
	return c.do(http.MethodPost, path, body, out)
}

func (c *client) put(path string, body any, out any) (int, error) {
	return c.do(http.MethodPut, path, body, out)
}

// rawPost sends a JSON POST and returns the raw response so callers can
// inspect error bodies.
func (c *client) rawPost(path string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, c.baseURI+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("X-AZ-API-KEY", c.apiKey)
	}
	return c.http.Do(request)
}

func jsonDecode(response *http.Response, out any) error {
	return json.NewDecoder(response.Body).Decode(out)
}
