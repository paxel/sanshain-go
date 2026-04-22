package api

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/paxel/sanshain/sanshain-go/internal/utils"
)

type ProvidePayload struct {
	ServiceName string `json:"servicename"`
	Branch      string `json:"branch"`
	OpenApiYaml string `json:"openapi_yaml"`
	DryRun      bool   `json:"dry_run,omitempty"`
	ApiType     string `json:"api_type,omitempty"`
}

type ProvideAsyncApiPayload struct {
	ServiceName  string `json:"servicename"`
	Branch       string `json:"branch"`
	AsyncApiYaml string `json:"asyncapi_yaml"`
	DryRun       bool   `json:"dry_run,omitempty"`
	ApiType      string `json:"api_type,omitempty"`
}

type ProvideProtoPayload struct {
	ServiceName  string `json:"servicename"`
	Branch       string `json:"branch"`
	ProtoContent string `json:"proto_content"`
	DryRun       bool   `json:"dry_run,omitempty"`
	ApiType      string `json:"api_type,omitempty"`
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
	ApiType     string                  `json:"api_type,omitempty"`
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
	if payload.ApiType == "" {
		payload.ApiType = "openapi"
	}
	return c.post("/provide", payload, compression)
}

func (c *SanshainClient) ProvideAsyncApi(payload ProvideAsyncApiPayload, compression bool) error {
	if payload.ApiType == "" {
		payload.ApiType = "asyncapi"
	}
	return c.post("/provide/asyncapi", payload, compression)
}

func (c *SanshainClient) ProvideProto(payload ProvideProtoPayload, compression bool) error {
	if payload.ApiType == "" {
		payload.ApiType = "proto"
	}
	return c.post("/provide/grpc", payload, compression)
}

func (c *SanshainClient) post(path string, payload interface{}, compression bool) error {
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

	req, err := http.NewRequest("POST", c.BaseURL+path, body)
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
		body, _ := c.readBody(resp)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
	}

	return nil
}

func (c *SanshainClient) Require(clientName, serviceName, branch, path, method string, timeout int, dryRun bool, apiType string) (string, error) {
	endpoint := "/require"
	if apiType == "asyncapi" {
		endpoint = "/require/asyncapi"
	} else if apiType == "proto" {
		endpoint = "/require/grpc"
	}

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return "", err
	}

	if apiType == "" {
		apiType = "openapi"
	}

	q := u.Query()
	q.Set("clientname", clientName)
	q.Set("servicename", serviceName)
	q.Set("branch", branch)
	q.Set("path", path)
	q.Set("method", method)
	q.Set("api_type", apiType)
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

	body, err := c.readBody(resp)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
	}

	return body, nil
}

func (c *SanshainClient) RequireBundle(payload RequireBundlePayload, compression bool) (string, error) {
	if payload.ApiType == "" {
		payload.ApiType = "openapi"
	}
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

	body, err := c.readBody(resp)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
	}

	return body, nil
}

func (c *SanshainClient) readBody(resp *http.Response) (string, error) {
	var reader io.ReadCloser
	var err error

	contentEncoding := resp.Header.Get("Content-Encoding")
	if contentEncoding == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer reader.Close()
	} else {
		reader = resp.Body
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

func sanitize(s string) string {
	if len(s) > 1000 {
		s = s[:1000] + "... (truncated)"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			return r
		}
		return '?'
	}, s)
}
