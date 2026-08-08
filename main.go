package main

// Version 由构建时 -ldflags "-X main.Version=..." 注入，release.yml 会根据用户输入填充。
var Version = "dev"

// metadata 是插件元数据定义。
// 注意：保留字段 key/cert 由 AllinSSL 在执行 apply 时注入，不会出现在 config 或 action params 中。
var metadata = PluginMetadata{
	Name:        "hydun-cdn",
	Description: "将证书部署到 Hydun CDN 域名的 HTTPS 配置",
	Version:     Version,
	Author:      "allinssl",
	Config: []ConfigParam{
		{
			Name:        "username",
			Type:        "string",
			Description: "Hydun CDN 用户名",
			Required:    true,
		},
		{
			Name:        "password",
			Type:        "string",
			Description: "Hydun CDN 密码",
			Required:    true,
		},
		{
			Name:        "verify_tls",
			Type:        "boolean",
			Description: "是否校验远端 TLS 证书",
			Required:    false,
		},
	},
	Actions: []ActionInfo{
		{
			Name:        "apply",
			Description: "将当前证书部署到指定的 Hydun CDN 域名",
			Params: []ConfigParam{
				{
					Name:        "domain_id",
					Type:        "string",
					Description: "目标域名 UUID",
					Required:    true,
				},
			},
		},
	},
}

func main() {
	debugf("插件启动，版本=%s", Version)

	req, err := readRequest()
	if err != nil {
		debugf("读取请求失败: %v", err)
		writeResponse(failure(err.Error()))
		return
	}
	debugf("收到 action=%s", req.Action)

	switch req.Action {
	case "get_metadata":
		// get_metadata 必须纯静态返回，不能访问网络或远端。
		writeResponse(success("元数据", metadataResult()))
	case "apply":
		resp, err := applyAction(req.Params)
		if err != nil {
			debugf("部署失败: %v", err)
			writeResponse(failure(err.Error()))
			return
		}
		debugf("部署成功: status=%s message=%s", resp.Status, resp.Message)
		writeResponse(resp)
	default:
		debugf("未知 action: %s", req.Action)
		writeResponse(failure("未知 action: " + req.Action))
	}
	debugf("插件退出")
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
