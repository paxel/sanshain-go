package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/paxel/sanshain/sanshain-go/internal/utils"
)

type ProvidePayload struct {
	ServiceName string `json:"servicename"`
	Branch      string `json:"branch"`
	OpenApiYaml string `json:"openapi_yaml"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

type RequireBundleEndpoint struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

type RequireBundlePayload struct {
	ClientName  string                  `json:"clientname"`
	ServiceName string                  `json:"servicename"`
	Branch      string                  `json:"branch"`
	Endpoints   []RequireBundleEndpoint `json:"endpoints"`
	Timeout     int                     `json:"timeout,omitempty"`
	DryRun      bool                    `json:"dry_run,omitempty"`
}

type SanshainClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewSanshainClient(baseURL, token string, insecure bool) *SanshainClient {
	httpClient := &http.Client{}
	if insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &SanshainClient{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: httpClient,
	}
}

func (c *SanshainClient) Provide(payload ProvidePayload, compression bool) error {
	var body io.Reader
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	if compression {
		compressed, err := utils.Compress(jsonData)
		if err != nil {
			return err
		}
		body = bytes.NewReader(compressed)
		header.Set("Content-Encoding", "gzip")
	} else {
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/provide", body)
	if err != nil {
		return err
	}
	req.Header = header
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *SanshainClient) Require(clientName, serviceName, branch, path, method string, timeout int, dryRun bool) (string, error) {
	u, err := url.Parse(c.BaseURL + "/require")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("clientname", clientName)
	q.Set("servicename", serviceName)
	q.Set("branch", branch)
	q.Set("path", path)
	q.Set("method", method)
	if timeout > 0 {
		q.Set("timeout", strconv.Itoa(timeout))
	}
	if dryRun {
		q.Set("dry_run", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return string(bodyBytes), nil
}

func (c *SanshainClient) RequireBundle(payload RequireBundlePayload, compression bool) (string, error) {
	var body io.Reader
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	if compression {
		compressed, err := utils.Compress(jsonData)
		if err != nil {
			return "", err
		}
		body = bytes.NewReader(compressed)
		header.Set("Content-Encoding", "gzip")
	} else {
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/require-bundle", body)
	if err != nil {
		return "", err
	}
	req.Header = header
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return string(bodyBytes), nil
}
