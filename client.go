package main

import (
	"crypto/tls"
	"net/http"
	"time"
)

// tls 被 action.go 引用，为避免在不同文件重复定义，统一在此导出一次。
// newHydunClient 在 action.go 中定义并使用本包的 tls.Config。

// defaultHTTPTimeout 是默认远端 API 超时。
const defaultHTTPTimeout = 15 * time.Second

// newTransport 根据是否校验 TLS 返回一个独立的传输层副本。
func newTransport(verifyTLS bool) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !verifyTLS {
		// 仅在授权配置明确关闭时才跳过 TLS 校验，默认保持校验以抵御中间人攻击。
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return transport
}
