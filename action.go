package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// applyAction 是证书部署主入口。
// 参数合并优先级（AllinSSL 控制）：host 注入的 key/cert > action params > access config。
func applyAction(params map[string]any) (Response, error) {
	if params == nil {
		return Response{}, fmt.Errorf("params are required")
	}
	debugf("applyAction invoked with %d params", len(params))

	// 读取授权配置（允许被 action params 覆盖）。
	endpoint, err := requireString(params, "endpoint")
	if err != nil {
		return Response{}, err
	}
	endpoint = strings.TrimRight(endpoint, "/")

	token, err := requireString(params, "credential")
	if err != nil {
		return Response{}, err
	}

	// 读取 action 参数。
	domainID, err := requireString(params, "domain_id")
	if err != nil {
		return Response{}, err
	}
	debugf("target endpoint=%s domain_id=%s", endpoint, domainID)

	// 读取 AllinSSL 注入的证书和私钥。
	certPEM, err := requireString(params, "cert")
	if err != nil {
		return Response{}, err
	}
	keyPEM, err := requireString(params, "key")
	if err != nil {
		return Response{}, err
	}
	debugf("received cert length=%d key length=%d", len(certPEM), len(keyPEM))

	verifyTLS := true
	if v, ok := params["verify_tls"].(bool); ok {
		verifyTLS = v
	}
	debugf("verify_tls=%v http_timeout=15s", verifyTLS)

	client := newHydunClient(endpoint, token, verifyTLS, 15*time.Second)

	if err := client.updateDomainCertificate(context.Background(), domainID, certPEM, keyPEM); err != nil {
		return Response{}, err
	}

	return success("certificate deployed to Hydun CDN domain", map[string]any{
		"domain_id": domainID,
	}), nil
}

// requireString 从 params 中提取非空字符串。
func requireString(params map[string]any, key string) (string, error) {
	value, ok := params[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required and must be a non-empty string", key)
	}
	return value, nil
}

// hydunClient 封装 Hydun CDN API 调用。
type hydunClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// newHydunClient 创建 Hydun API 客户端。
func newHydunClient(baseURL, token string, verifyTLS bool, timeout time.Duration) *hydunClient {
	return &hydunClient{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: timeout, Transport: newTransport(verifyTLS)},
	}
}

// updateDomainCertificate 调用 Hydun API 更新指定域名的 HTTPS 证书。
// 对应接口：PUT /api/v1/domains/:id/certificate，body 使用 cert_pem + key_pem。
func (c *hydunClient) updateDomainCertificate(ctx context.Context, domainID, certPEM, keyPEM string) error {
	debugf("building PUT /api/v1/domains/%s/certificate request", domainID)
	payload := map[string]string{
		"cert_pem": certPEM,
		"key_pem":  keyPEM,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/domains/%s/certificate", c.baseURL, domainID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Hydun API: %w", err)
	}
	defer resp.Body.Close()
	debugf("Hydun API responded status=%s", resp.Status)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	// 尝试解析统一响应格式 { code, message, data }，失败时回退原始片段。
	var apiErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
		return fmt.Errorf("Hydun API error %d: %s", resp.StatusCode, apiErr.Message)
	}
	return fmt.Errorf("Hydun API returned HTTP %d: %s", resp.StatusCode, string(respBody))
}
