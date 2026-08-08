package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultEndpoint 是 Hydun CDN API 的固定地址。
const defaultEndpoint = "https://kzt.hydun.com"

// applyAction 是证书部署主入口。
// 参数合并优先级（AllinSSL 控制）：host 注入的 key/cert > action params > access config。
func applyAction(params map[string]any) (Response, error) {
	if params == nil {
		return Response{}, fmt.Errorf("缺少参数")
	}
	debugf("applyAction 被调用，共 %d 个参数", len(params))

	// 读取授权配置。
	username, err := requireString(params, "username")
	if err != nil {
		return Response{}, fmt.Errorf("用户名: %w", err)
	}
	password, err := requireString(params, "password")
	if err != nil {
		return Response{}, fmt.Errorf("密码: %w", err)
	}

	// 读取 action 参数。
	domainID, err := requireString(params, "domain_id")
	if err != nil {
		return Response{}, err
	}
	debugf("目标 domain_id=%s", domainID)

	// 读取 AllinSSL 注入的证书和私钥。
	certPEM, err := requireString(params, "cert")
	if err != nil {
		return Response{}, err
	}
	keyPEM, err := requireString(params, "key")
	if err != nil {
		return Response{}, err
	}
	debugf("收到证书长度=%d 私钥长度=%d", len(certPEM), len(keyPEM))

	verifyTLS := true
	if v, ok := params["verify_tls"].(bool); ok {
		verifyTLS = v
	}
	debugf("verify_tls=%v http_timeout=15s", verifyTLS)

	client := newHydunClient(defaultEndpoint, verifyTLS, 15*time.Second)

	debugf("正在登录 Hydun CDN 获取 token")
	token, err := client.login(context.Background(), username, password)
	if err != nil {
		return Response{}, err
	}
	debugf("登录成功")
	client.token = token

	if err := client.updateDomainCertificate(context.Background(), domainID, certPEM, keyPEM); err != nil {
		return Response{}, err
	}

	return success("证书已部署到 Hydun CDN 域名", map[string]any{
		"domain_id": domainID,
	}), nil
}

// requireString 从 params 中提取非空字符串。
func requireString(params map[string]any, key string) (string, error) {
	value, ok := params[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s 必填且必须为非空字符串", key)
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
func newHydunClient(baseURL string, verifyTLS bool, timeout time.Duration) *hydunClient {
	return &hydunClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout, Transport: newTransport(verifyTLS)},
	}
}

// login 调用 Hydun CDN 登录接口获取 JWT token。
// 对应接口：POST /api/v1/auth/login，body 使用 username + password。
func (c *hydunClient) login(ctx context.Context, username, password string) (string, error) {
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化登录请求: %w", err)
	}

	url := c.baseURL + "/api/v1/auth/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构造登录请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用登录接口: %w", err)
	}
	defer resp.Body.Close()
	debugf("登录接口返回状态=%s", resp.Status)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("读取登录响应: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", parseAPIError("登录失败", resp.StatusCode, respBody)
	}

	// Hydun CDN 统一响应格式: { code, message, data }，登录成功时 data 应为 token 字符串。
	var loginResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return "", fmt.Errorf("解析登录响应: %w", err)
	}
	if loginResp.Code != 0 {
		return "", fmt.Errorf("登录失败: %s", loginResp.Message)
	}
	if loginResp.Data == "" {
		return "", fmt.Errorf("登录响应中未包含 token")
	}
	return loginResp.Data, nil
}

// updateDomainCertificate 调用 Hydun API 更新指定域名的 HTTPS 证书。
// 对应接口：PUT /api/v1/domains/:id/certificate，body 使用 cert_pem + key_pem。
func (c *hydunClient) updateDomainCertificate(ctx context.Context, domainID, certPEM, keyPEM string) error {
	debugf("构造 PUT /api/v1/domains/%s/certificate 请求", domainID)
	payload := map[string]string{
		"cert_pem": certPEM,
		"key_pem":  keyPEM,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化证书请求: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/domains/%s/certificate", c.baseURL, domainID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造证书请求: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("调用证书接口: %w", err)
	}
	defer resp.Body.Close()
	debugf("证书接口返回状态=%s", resp.Status)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("读取证书响应: %w", err)
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	return parseAPIError("部署证书失败", resp.StatusCode, respBody)
}

// parseAPIError 尝试解析 Hydun CDN 统一响应格式，失败时回退原始片段。
func parseAPIError(prefix string, statusCode int, body []byte) error {
	var apiErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
		return fmt.Errorf("%s (HTTP %d): %s", prefix, statusCode, apiErr.Message)
	}
	return fmt.Errorf("%s (HTTP %d): %s", prefix, statusCode, string(body))
}
