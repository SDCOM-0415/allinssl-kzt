//go:build !debug

package main

// Debug 在 release 构建时为 false，stdout 保持干净以符合 AllinSSL 协议。
const Debug = false
