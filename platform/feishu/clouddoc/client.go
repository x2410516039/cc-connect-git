package clouddoc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 20 * time.Second
const tenantTokenAPIPath = "/open-apis/auth/v3/tenant_access_token/internal"

type Client struct {
	platform    string
	baseURL     string
	appID       string
	appSecret   string
	http        *http.Client
	tenantToken string
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	appID := strings.TrimSpace(cfg.AppID)
	appSecret := strings.TrimSpace(cfg.AppSecret)
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("app_id and app_secret are required")
	}
	hc := cfg.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	platform := strings.TrimSpace(cfg.Platform)
	if platform == "" {
		platform = "feishu"
	}
	return &Client{
		platform:  platform,
		baseURL:   baseURL,
		appID:     appID,
		appSecret: appSecret,
		http:      hc,
	}, nil
}

func (c *Client) Platform() string {
	return c.platform
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) Call(ctx context.Context, method, apiPath string, query url.Values, body any) (*Response, error) {
	if strings.TrimSpace(c.tenantToken) == "" {
		if err := c.RefreshTenantToken(ctx); err != nil {
			return nil, err
		}
	}
	res, err := c.callWithToken(ctx, method, apiPath, query, body, c.tenantToken)
	if !IsInvalidTenantToken(err) {
		return res, err
	}
	if err := c.RefreshTenantToken(ctx); err != nil {
		return nil, err
	}
	return c.callWithToken(ctx, method, apiPath, query, body, c.tenantToken)
}

func (c *Client) RefreshTenantToken(ctx context.Context) error {
	payload := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return fmt.Errorf("encode tenant token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+tenantTokenAPIPath, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch tenant access token: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read tenant token response: %w", err)
	}
	logID := responseLogID(resp, raw)
	if resp.StatusCode >= 400 {
		return &APIError{Type: "http_error", APIPath: tenantTokenAPIPath, Message: string(raw), HTTPStatus: resp.StatusCode, LogID: logID}
	}

	var parsed tenantTokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("decode tenant token response: %w", err)
	}
	if parsed.LogID != "" {
		logID = parsed.LogID
	}
	if parsed.Code != 0 {
		return &APIError{Type: "api_error", APIPath: tenantTokenAPIPath, Message: parsed.Msg, Code: parsed.Code, LogID: logID, HTTPStatus: resp.StatusCode}
	}
	if strings.TrimSpace(parsed.TenantAccessToken) == "" {
		return &APIError{Type: "api_error", APIPath: tenantTokenAPIPath, Message: "tenant token response returned empty token", LogID: logID, HTTPStatus: resp.StatusCode}
	}
	c.tenantToken = strings.TrimSpace(parsed.TenantAccessToken)
	return nil
}

func (c *Client) callWithToken(ctx context.Context, method, apiPath string, query url.Values, body any, token string) (*Response, error) {
	rawURL := c.baseURL + apiPath
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = &buf
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	logID := responseLogID(resp, raw)

	var envelope apiEnvelope
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		if envelope.LogID != "" {
			logID = envelope.LogID
		}
	}
	if resp.StatusCode >= 400 {
		msg := envelope.Msg
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return nil, &APIError{Type: "http_error", APIPath: apiPath, Message: msg, Code: envelope.Code, LogID: logID, HTTPStatus: resp.StatusCode}
	}
	if envelope.Code != 0 {
		return nil, &APIError{Type: "api_error", APIPath: apiPath, Message: envelope.Msg, Code: envelope.Code, LogID: logID, HTTPStatus: resp.StatusCode}
	}

	data := map[string]any{}
	if len(bytes.TrimSpace(envelope.Data)) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return nil, fmt.Errorf("decode response data: %w", err)
		}
	}
	return &Response{Data: data, LogID: logID, HTTPStatus: resp.StatusCode}, nil
}

func responseLogID(resp *http.Response, body []byte) string {
	for _, key := range []string{"X-Tt-Logid", "X-Request-Id", "X-Lark-Request-Id"} {
		if v := strings.TrimSpace(resp.Header.Get(key)); v != "" {
			return v
		}
	}
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	if v, ok := probe["log_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
