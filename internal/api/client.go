package api

import (
	"bytes"
	"cmp"
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

	"github.com/paxel/sanshain-go/v2/internal/utils"
)

// ProvidePayload is the 2.0 wire payload for POST /provide.
// The version is not part of the payload — the server reads it from the
// spec's info.version.
type ProvidePayload struct {
	ProducerName string `json:"producername"`
	OpenApiYaml  string `json:"openapi_yaml"`
	Stability    string `json:"stability"`
	DryRun       bool   `json:"dry_run,omitempty"`
	// The stream this build belongs to (ADR-0004/0005). Omitted when
	// undeclared — the server defaults trunk to false and treats an absent
	// tag as "no branch". Mutually exclusive; the server answers 400 for both.
	Trunk bool   `json:"trunk,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

type ProvideAsyncApiPayload struct {
	ProducerName string `json:"producername"`
	AsyncApiYaml string `json:"asyncapi_yaml"`
	Stability    string `json:"stability"`
	DryRun       bool   `json:"dry_run,omitempty"`
	Trunk        bool   `json:"trunk,omitempty"`
	Tag          string `json:"tag,omitempty"`
}

type ProvideProtoPayload struct {
	ProducerName string `json:"producername"`
	ProtoContent string `json:"proto_content"`
	Stability    string `json:"stability"`
	DryRun       bool   `json:"dry_run,omitempty"`
	Trunk        bool   `json:"trunk,omitempty"`
	Tag          string `json:"tag,omitempty"`
}

// RetirePayload declares that the Producer no longer provides an API family —
// an ordinary provide call carrying `retired: true` and no document. It has no
// field for a body, a stability or a stream: the server answers 400 for a
// request that says both, and a payload that cannot express the contradiction
// cannot send it.
type RetirePayload struct {
	ProducerName string `json:"producername"`
	Retired      bool   `json:"retired"`
	DryRun       bool   `json:"dry_run,omitempty"`
}

// RetiredProtocol is what retiring shed. A retire publishes no version, so it
// reports none; every count is zero under dry_run.
type RetiredProtocol struct {
	// The capability tag removed (messaging/grpc); empty for OpenAPI.
	TagCleared string `json:"tag_cleared"`
	// Open trunk pins closed — they leave the current main graph, their
	// closed rows stay as timeline history.
	TrunkPinsClosed int64 `json:"trunk_pins_closed"`
	// AsyncAPI channel-message contracts released for another Producer.
	ContractsReleased int64 `json:"contracts_released"`
}

type ProvideResponse struct {
	Version     string         `json:"version"`
	Stability   string         `json:"stability"`
	ContentHash string         `json:"content_hash"`
	Changes     ProvideChanges `json:"changes"`
	// AsyncAPI subscribe operations harvested from this document as
	// version-less consumer edges; only AsyncAPI provides that declare any.
	HarvestedSubscriptions []HarvestedSubscription `json:"harvested_subscriptions"`
}

// HarvestedSubscription is one AsyncAPI subscribe operation harvested from a
// provide, checked against the publishing Producer's GA channel contract.
// Expecting a field the contract does not guarantee is drift — reported here on
// a snapshot, refused with 409 on a GA provide.
type HarvestedSubscription struct {
	Channel     string `json:"channel"`
	MessageName string `json:"message_name"`
	// The Producer owning the channel's PUB contract; empty while none does.
	Owner string `json:"owner"`
	// Why the expectation is not satisfiable by the current contract.
	Drift string `json:"drift"`
}

// Advisory reports whether this subscription deserves a warning rather than an
// informational line: drift against the contract, or no publisher at all.
func (h HarvestedSubscription) Advisory() bool {
	return h.Drift != "" || h.Owner == ""
}

// Describe renders the one-line build-log form.
func (h HarvestedSubscription) Describe() string {
	line := h.Channel + " / " + h.MessageName
	if h.Owner != "" {
		line += " <- " + h.Owner
	} else {
		line += " <- (no publisher yet)"
	}
	if h.Drift != "" {
		line += " — " + h.Drift
	}
	return line
}

type ProvideChanges struct {
	Inserts int `json:"inserts"`
	Updates int `json:"updates"`
	Deletes int `json:"deletes"`
}

type RequireResult struct {
	Content     string
	Etag        string
	NotModified bool
}

type RequireBundleEndpoint struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

// RequireBundlePayload is the wire payload for POST /require-bundle. The
// stream travels in the body on this endpoint — the server's bundle handler
// reads no query parameters, so a stream on the query string would be silently
// dropped and a trunk build would record no trunk pins.
type RequireBundlePayload struct {
	ConsumerName string                  `json:"consumername"`
	ProducerName string                  `json:"producername"`
	Version      string                  `json:"version"`
	ApiType      string                  `json:"api_type,omitempty"`
	Endpoints    []RequireBundleEndpoint `json:"endpoints"`
	DryRun       bool                    `json:"dry_run,omitempty"`
	Trunk        bool                    `json:"trunk,omitempty"`
	Tag          string                  `json:"tag,omitempty"`
}

// Stream names which dependency graph a build's calls belong to: trunk, one
// sanshain-branch (Tag), or neither (the zero value).
//
// The stream is a property of the invocation, not of the repository, so it is
// declared by the pipeline (--trunk / SANSHAIN_TRUNK, --tag / SANSHAIN_TAG)
// and never committed to sanshain.yaml: a developer's laptop and the trunk CI
// job build the same checkout, and the trunk pin store is last-writer-wins, so
// inferring the stream would let a local build overwrite what CI recorded.
type Stream struct {
	Trunk bool
	Tag   string
}

// ResolveStream resolves the declared stream from an explicit flag pair and
// the SANSHAIN_TRUNK / SANSHAIN_TAG environment, preferring the flags. A build
// declaring both trunk and a tag is refused here — the server would answer 400,
// and naming the misconfiguration locally beats spending a round trip on it.
func ResolveStream(trunkFlag bool, tagFlag, trunkEnv, tagEnv string) (Stream, error) {
	trunk := trunkFlag || trunkEnv == "true" || trunkEnv == "1"
	tag := strings.TrimSpace(tagFlag)
	if tag == "" {
		tag = strings.TrimSpace(tagEnv)
	}
	if trunk && tag != "" {
		return Stream{}, fmt.Errorf(
			"this build declares both trunk and tag '%s' — a build belongs to the trunk stream or to one sanshain-branch, never both; set only one of --trunk / SANSHAIN_TRUNK and --tag / SANSHAIN_TAG",
			tag)
	}
	return Stream{Trunk: trunk, Tag: tag}, nil
}

// ConflictError is a 409 rejection by the server's version rules.
// ProposedVersion carries the next free version to publish as instead.
type ConflictError struct {
	Message         string
	ProposedVersion string
}

func (e *ConflictError) Error() string {
	if e.ProposedVersion != "" {
		return fmt.Sprintf("%s (proposed version: %s)", e.Message, e.ProposedVersion)
	}
	return e.Message
}

// errorBody is the JSON error shape every non-2xx Sanshain response carries.
type errorBody struct {
	Error           string `json:"error"`
	ProposedVersion string `json:"proposed_version"`
}

func parseErrorBody(body string) errorBody {
	var eb errorBody
	_ = json.Unmarshal([]byte(body), &eb)
	if eb.Error == "" {
		eb.Error = sanitize(body)
	}
	return eb
}

type SanshainClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client

	serverChecked  bool
	preTwoDiagnose error
}

func NewSanshainClient(baseURL, token string, insecure bool) *SanshainClient {
	httpClient := &http.Client{}
	if insecure {
		httpClient.Transport = &http.Transport{
			/* #nosec G402 */
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &SanshainClient{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: httpClient,
	}
}

func (c *SanshainClient) Provide(payload ProvidePayload, compression bool) (*ProvideResponse, error) {
	return c.postProvide("/provide", payload, compression)
}

func (c *SanshainClient) ProvideAsyncApi(payload ProvideAsyncApiPayload, compression bool) (*ProvideResponse, error) {
	return c.postProvide("/provide/asyncapi", payload, compression)
}

func (c *SanshainClient) ProvideProto(payload ProvideProtoPayload, compression bool) (*ProvideResponse, error) {
	return c.postProvide("/provide/grpc", payload, compression)
}

// Retire declares that the Producer no longer provides the given API family.
// Retiring is gated like releasing: the caller needs the 'releaser' role — the
// role a release pipeline already holds — or a maintainer grant on this
// Producer, or the server answers 403.
func (c *SanshainClient) Retire(producerName, apiType string, dryRun bool) (*RetiredProtocol, error) {
	path := "/provide"
	if apiType == "asyncapi" {
		path = "/provide/asyncapi"
	} else if apiType == "proto" || apiType == "grpc" {
		path = "/provide/grpc"
	}

	resp, err := c.post(path, RetirePayload{ProducerName: producerName, Retired: true, DryRun: dryRun}, false, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.retireFailure(resp, body, producerName)
	}

	var result RetiredProtocol
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetServerVersion reads the running instance's own build version
// from GET /version.
func (c *SanshainClient) GetServerVersion() (string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/version", nil)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("GET /version failed with status %d", resp.StatusCode)
	}

	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// preTwoServerError lazily checks (at most once per client) whether the
// server is a pre-2.0 Sanshain instance. It returns a replacement error
// when it is, and nil otherwise (including when the check itself fails).
func (c *SanshainClient) preTwoServerError() error {
	if !c.serverChecked {
		c.serverChecked = true
		v, err := c.GetServerVersion()
		if err == nil {
			if major, ok := parseMajor(v); ok && major < 2 {
				c.preTwoDiagnose = fmt.Errorf(
					"Sanshain server at %s is %s; this client requires Sanshain 2.x — upgrade the server",
					c.BaseURL, v)
			}
		}
	}
	return c.preTwoDiagnose
}

func parseMajor(version string) (int, bool) {
	part, _, _ := strings.Cut(strings.TrimSpace(version), ".")
	major, err := strconv.Atoi(part)
	if err != nil {
		return 0, false
	}
	return major, true
}

// provideFailure turns a failed provide response into an error, diagnosing
// pre-2.0 servers and surfacing 409 version-rule rejections.
func (c *SanshainClient) provideFailure(resp *http.Response, body string) error {
	if err := c.preTwoServerError(); err != nil {
		return err
	}
	eb := parseErrorBody(body)
	if resp.StatusCode == http.StatusConflict {
		return &ConflictError{Message: eb.Error, ProposedVersion: eb.ProposedVersion}
	}
	// Releasing is a permission, not merely an authenticated act, and the
	// remedy is a role grant — nothing the Producer can change in its own
	// repository. Naming it is the whole value of catching this status.
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf(
			"provide refused (403): %s\n  Publishing a GA version requires the 'releaser' role. Grant it to the user or token this build authenticates as (administrators and root always hold it), or publish as a snapshot by leaving --ga / SANSHAIN_GA unset",
			eb.Error)
	}
	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
}

// retireFailure names both ways into the retire gate — the releaser role a
// pipeline holds, and a maintainer grant on the Producer.
func (c *SanshainClient) retireFailure(resp *http.Response, body, producerName string) error {
	if err := c.preTwoServerError(); err != nil {
		return err
	}
	eb := parseErrorBody(body)
	switch resp.StatusCode {
	case http.StatusForbidden:
		return fmt.Errorf(
			"retire refused (403): %s\n  Retiring an API family requires the 'releaser' role — which a release pipeline already holds — or a maintainer grant on '%s'. Administrators and root hold both",
			eb.Error, producerName)
	case http.StatusNotFound:
		return fmt.Errorf(
			"retire failed (404): %s\n  Nothing was ever provided as '%s', so there is no family to retire",
			eb.Error, producerName)
	}
	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
}

// requireFailure turns a failed require response into an error, diagnosing
// pre-2.0 servers and keeping 404 (unknown) distinct from 410 (absent).
func (c *SanshainClient) requireFailure(resp *http.Response, body, producerName, version string) error {
	if err := c.preTwoServerError(); err != nil {
		return err
	}
	eb := parseErrorBody(body)
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf(
			"unknown producer or version (404): %s — the pin %s@%s does not exist on the server; fix the version in sanshain.yaml (list available: GET /producers/%s/versions)",
			eb.Error, producerName, version, producerName)
	case http.StatusGone:
		return fmt.Errorf(
			"endpoint absent (410): version %s of %s exists but deliberately lacks the requested endpoint(s): %s",
			version, producerName, eb.Error)
	}
	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, sanitize(body))
}

func (c *SanshainClient) post(path string, payload any, compression bool, extraHeaders http.Header) (*http.Response, error) {
	var body io.Reader
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		header[k] = v
	}

	if compression {
		compressed, err := utils.Compress(jsonData)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(compressed)
		header.Set("Content-Encoding", "gzip")
	} else {
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header = header
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	return c.HTTPClient.Do(req)
}

func (c *SanshainClient) postProvide(path string, payload any, compression bool) (*ProvideResponse, error) {
	resp, err := c.post(path, payload, compression, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := c.readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.provideFailure(resp, respBody)
	}

	var provideResp ProvideResponse
	if err := json.Unmarshal([]byte(respBody), &provideResp); err != nil {
		return nil, err
	}
	return &provideResp, nil
}

func (c *SanshainClient) Require(consumerName, producerName, version, path, method string, dryRun bool, apiType string) (string, error) {
	result, err := c.RequireWithEtag(consumerName, producerName, version, path, method, dryRun, apiType, "", Stream{})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (c *SanshainClient) RequireWithEtag(consumerName, producerName, version, reqPath, method string, dryRun bool, apiType string, etag string, stream Stream) (*RequireResult, error) {
	endpoint := "/require"
	if apiType == "asyncapi" {
		endpoint = "/require/asyncapi"
	} else if apiType == "proto" {
		endpoint = "/require/grpc"
	}

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("consumername", consumerName)
	q.Set("producername", producerName)
	q.Set("version", version)
	q.Set("path", reqPath)
	q.Set("method", method)
	if dryRun {
		q.Set("dry_run", "true")
	}
	// Unlike the bundle, the single-endpoint require reads its stream from
	// the query string.
	if stream.Trunk {
		q.Set("trunk", "true")
	} else if stream.Tag != "" {
		q.Set("tag", stream.Tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &RequireResult{NotModified: true}, nil
	}

	body, err := c.readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.requireFailure(resp, body, producerName, version)
	}

	responseEtag := resp.Header.Get("ETag")
	return &RequireResult{Content: body, Etag: responseEtag, NotModified: false}, nil
}

func (c *SanshainClient) RequireBundle(payload RequireBundlePayload, compression bool) (string, error) {
	result, err := c.RequireBundleWithEtag(payload, compression, "")
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (c *SanshainClient) RequireBundleWithEtag(payload RequireBundlePayload, compression bool, etag string) (*RequireResult, error) {
	payload.ApiType = cmp.Or(payload.ApiType, "openapi")
	headers := make(http.Header)
	if etag != "" {
		headers.Set("If-None-Match", etag)
	}

	resp, err := c.post("/require-bundle", payload, compression, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &RequireResult{NotModified: true}, nil
	}

	respBody, err := c.readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.requireFailure(resp, respBody, payload.ProducerName, payload.Version)
	}

	return &RequireResult{
		Content:     respBody,
		Etag:        resp.Header.Get("ETag"),
		NotModified: false,
	}, nil
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

// NormalizeLineEndings rewrites CRLF (and stray CR) to LF. Sanshain compares
// provided content byte for byte, so a Windows checkout of an otherwise
// identical file would hash differently and provoke a spurious 409.
func NormalizeLineEndings(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
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
