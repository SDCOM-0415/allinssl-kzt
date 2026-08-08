package main

import (
	"fmt"
	"os"
)

// debugf 仅在 Debug 构建时向 stderr 输出日志。
// 用于 GitHub Actions 工作流执行日志排查，不会污染 AllinSSL 的 stdout 协议通道。
func debugf(format string, args ...any) {
	if Debug {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}
