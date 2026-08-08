package main

// Version 由构建时 -ldflags "-X main.Version=..." 注入，release.yml 会根据用户输入填充。
var Version = "dev"

// metadata 是插件元数据定义。
// 注意：保留字段 key/cert 由 AllinSSL 在执行 apply 时注入，不会出现在 config 或 action params 中。
var metadata = PluginMetadata{
	Name:        "hydun-cdn",
	Description: "Deploy certificates to Hydun CDN domain HTTPS configuration",
	Version:     Version,
	Author:      "allinssl",
	Config: []ConfigParam{
		{
			Name:        "endpoint",
			Type:        "string",
			Description: "Hydun API base URL, e.g. https://kzt.hydun.com",
			Required:    true,
		},
		{
			Name:        "credential",
			Type:        "string",
			Description: "Hydun API JWT token (Authorization: Bearer <token>)",
			Required:    true,
		},
		{
			Name:        "verify_tls",
			Type:        "boolean",
			Description: "Verify remote TLS certificate",
			Required:    false,
		},
	},
	Actions: []ActionInfo{
		{
			Name:        "apply",
			Description: "Deploy the current certificate to a Hydun CDN domain",
			Params: []ConfigParam{
				{
					Name:        "domain_id",
					Type:        "string",
					Description: "Target domain UUID",
					Required:    true,
				},
			},
		},
	},
}

func main() {
	debugf("plugin started, version=%s", Version)

	req, err := readRequest()
	if err != nil {
		debugf("failed to read request: %v", err)
		writeResponse(failure(err.Error()))
		return
	}
	debugf("received action=%s", req.Action)

	switch req.Action {
	case "get_metadata":
		// get_metadata 必须纯静态返回，不能访问网络或远端。
		writeResponse(success("metadata", metadataResult()))
	case "apply":
		resp, err := applyAction(req.Params)
		if err != nil {
			debugf("apply failed: %v", err)
			writeResponse(failure(err.Error()))
			return
		}
		debugf("apply succeeded: status=%s message=%s", resp.Status, resp.Message)
		writeResponse(resp)
	default:
		debugf("unknown action: %s", req.Action)
		writeResponse(failure("unknown action: " + req.Action))
	}
	debugf("plugin exiting")
}

// metadataResult 把强类型元数据转换为 map 以返回 AllinSSL。
func metadataResult() map[string]any {
	return map[string]any{
		"name":        metadata.Name,
		"description": metadata.Description,
		"version":     metadata.Version,
		"author":      metadata.Author,
		"actions":     metadata.Actions,
		"config":      metadata.Config,
	}
}
