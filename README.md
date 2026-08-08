# Hydun CDN Plugin for AllinSSL

AllinSSL 1.1.3 可执行插件，用于将 SSL/TLS 证书自动部署到 [Hydun CDN](https://kzt.hydun.com) 域名的 HTTPS 配置。

## 工作原理

插件通过 AllinSSL 的 stdin/stdout JSON 协议接收证书部署请求，调用 Hydun CDN API `PUT /api/v1/domains/:id/certificate` 将证书和私钥更新到指定域名。

```
AllinSSL → stdin (JSON) → 插件进程 → HTTP PUT → Hydun CDN API
                                ↓
                    stdout (JSON) → AllinSSL
```

## 插件元数据

| 字段       | 值                                                     |
| ---------- | ------------------------------------------------------ |
| name       | `hydun-cdn`                                            |
| version    | 构建时通过 `-ldflags` 注入                             |
| actions    | `apply` — 部署证书到指定域名                           |

### 授权配置 (config)

| 字段         | 类型    | 必填 | 说明                                              |
| ------------ | ------- | ---- | ------------------------------------------------- |
| `endpoint`   | string  | 是   | Hydun API 地址，如 `https://kzt.hydun.com`       |
| `credential` | string  | 是   | JWT Token，用于 `Authorization: Bearer <token>`   |
| `verify_tls` | boolean | 否   | 是否校验远端 TLS 证书，默认 `true`                |

### Action 参数 (params)

**apply**

| 字段        | 类型   | 必填 | 说明             |
| ----------- | ------ | ---- | ---------------- |
| `domain_id` | string | 是   | 目标域名 UUID    |

> `cert` 和 `key` 由 AllinSSL 在执行部署前自动注入，无需用户填写。

## 文件结构

```
.
├── main.go              # 入口：stdin 读取、action 分发、stdout 响应
├── protocol.go          # Request/Response/Metadata 类型定义与 JSON 辅助
├── action.go            # apply action 参数校验与 Hydun API 调用逻辑
├── client.go            # HTTP 传输层与 TLS 配置
├── logger.go            # debugf 日志辅助函数
├── debug.go             # debug 构建标签 (Debug = true)
├── release.go           # release 构建标签 (Debug = false)
├── go.mod               # Go 模块定义
├── Makefile             # 本地构建与测试目标
├── scripts/
│   └── validate_plugin.py  # AllinSSL 1.1.3 插件协议校验脚本
└── .github/workflows/
    └── release.yml      # GitHub Actions 构建发布工作流
```

## 构建版本说明

| 版本    | 构建参数            | 行为                                                   |
| ------- | ------------------- | ------------------------------------------------------ |
| release | 默认（无额外标签）  | 符号剥离 (`-s -w`)，无 stderr 日志，适合生产环境       |
| debug   | `-tags debug`       | 保留符号，运行时向 stderr 输出完整调试日志             |

debug 版本的日志示例：

```
[debug] plugin started, version=1.0.0-debug
[debug] received action=apply
[debug] applyAction invoked with 5 params
[debug] target endpoint=https://kzt.hydun.com domain_id=uuid
[debug] received cert length=3568 key length=1678
[debug] verify_tls=true http_timeout=15s
[debug] building PUT /api/v1/domains/uuid/certificate request
[debug] Hydun API responded status=200 OK
[debug] apply succeeded: status=success message=certificate deployed
[debug] plugin exiting
```

> debug 日志只输出到 stderr，不会污染 AllinSSL 的 stdout 协议通道。但 AllinSSL 1.1.3 在 action 调用时会丢弃 stderr，因此 debug 版本主要用于本地直接运行或 CI 日志排查。

## 本地构建

### 前置要求

- Go 1.23+

### 构建命令

```bash
# release 版本（当前平台）
make build

# debug 版本（当前平台）
make build-debug

# 指定版本号
make build VERSION=1.0.0
make build-debug VERSION=1.0.0
```

### 交叉编译

```bash
# Windows x86
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=1.0.0" -o dist/hydun-cdn-windows-amd64.exe .

# Linux x86
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=1.0.0" -o dist/hydun-cdn-linux-amd64 .

# macOS ARM
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.Version=1.0.0" -o dist/hydun-cdn-darwin-arm64 .
```

## 本地测试

### 协议校验

```bash
# 校验 metadata
python scripts/validate_plugin.py ./dist/hydun-cdn

# 校验 apply action（期望失败，因为 credential 无效）
python scripts/validate_plugin.py ./dist/hydun-cdn \
  --action apply \
  --params '{"endpoint":"https://kzt.hydun.com","credential":"fake","domain_id":"uuid","cert":"PEM","key":"PEM"}' \
  --expect-error
```

### 手动测试

```bash
# 获取元数据
echo '{"action":"get_metadata"}' | ./dist/hydun-cdn

# 测试 apply（debug 版本可看到完整日志）
echo '{"action":"apply","params":{"endpoint":"https://kzt.hydun.com","credential":"YOUR_TOKEN","domain_id":"DOMAIN_UUID","cert":"CERT_PEM","key":"KEY_PEM"}}' | ./dist/hydun-cdn-debug
```

## GitHub Actions 发布

### 触发方式

在 GitHub 仓库的 **Actions** 页面选择 `Release Hydun CDN Plugin` 工作流，点击 **Run workflow** 按钮。

### 输入参数

| 参数             | 类型   | 说明                           | 示例       |
| ---------------- | ------ | ------------------------------ | ---------- |
| `tag`            | string | Release 标签                   | `v1.0.0`  |
| `version`        | string | 插件元数据中的版本号           | `1.0.0`   |
| `release_status` | choice | 发布状态：`latest` 或 `pre-release` | `latest` |

### 构建矩阵

| 平台    | Runner           | GOOS    | GOARCH | 产物名                               |
| ------- | ---------------- | ------- | ------ | ------------------------------------ |
| Windows | windows-latest   | windows | amd64  | `hydun-cdn-windows-amd64.exe`       |
| Linux   | ubuntu-latest    | linux   | amd64  | `hydun-cdn-linux-amd64`             |
| macOS   | macos-latest     | darwin  | arm64  | `hydun-cdn-darwin-arm64`            |

每个平台同时构建 release 和 debug 两个版本，共 6 个产物。

### 发布产物

- 6 个二进制文件
- `checksums.txt`（SHA-256 校验和）
- 根据选择的发布状态设置为 latest 或 pre-release

## 接入 AllinSSL

1. 从 GitHub Release 下载与 AllinSSL 服务器 OS/ARCH 匹配的 release 版本二进制
2. 将二进制放入 AllinSSL 的插件目录（默认 `plugins`）
3. POSIX 系统设置执行权限：
   ```bash
   chmod +x hydun-cdn-linux-amd64
   ```
4. 在 AllinSSL 管理界面触发插件扫描，确认 `hydun-cdn` 出现在插件列表
5. 新建 `plugin` 类型的主机授权，选择 `hydun-cdn`，填写：
   - `endpoint`：Hydun API 地址
   - `credential`：JWT Token
6. 在证书部署工作流中选择该授权和 `apply` action，填写 `domain_id`

## 安全说明

- 默认校验远端 TLS 证书，仅在授权配置中显式关闭 `verify_tls` 时才跳过
- 所有 HTTP 请求设置 15 秒超时
- 不会在响应中回显证书、私钥、Token 等敏感信息
- stdout 只输出最终 JSON 响应，不混入日志
- 不执行外部 shell 命令，不依赖工作目录

## 兼容性

- AllinSSL 1.1.3 插件协议
- Go 1.23+，CGO_ENABLED=0 静态编译
- 无第三方依赖，仅使用 Go 标准库
