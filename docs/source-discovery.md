# 代理源发现爬虫设计

这个辅助爬虫用于发现“代理源”，不是直接发现“可用代理”。它像一个侦察员：去 GitHub 和公开页面里找可能的代理列表 URL，然后交给 plugproxy 的采集、检测、评分流程处理。

## 为什么需要

免费代理源变化非常快。很多项目会维护自己的 `sources/` 目录、README 下载链接、Raw 文件或 API 示例。手工整理可以启动项目，但长期维护需要自动发现和周期性复核。

## 输入

- GitHub 搜索关键词。
- GitHub 仓库地址。
- 已知代理源 URL。
- 已知源清单文件，例如 `sources/http.txt`。

## 输出

输出候选代理源，不直接写入默认启用配置。

```json
{
  "name": "vakhov-http",
  "url": "https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/http.txt",
  "format": "text",
  "protocol_hint": "http",
  "confidence": 0.8,
  "status": "candidate",
  "discovered_from": "github:gfpcom/free-proxy-list:sources/http.txt"
}
```

## 发现策略

### GitHub 仓库搜索

关键词：

- `free proxy list`
- `proxy scraper`
- `socks5.txt`
- `sources/http.txt`
- `raw.githubusercontent.com proxy socks4`

优先检查：

- README。
- `sources/`。
- `proxies/`。
- `.github/workflows/`。
- 常见配置文件，例如 `.json`、`.yaml`、`.txt`。

### URL 抽取

识别这些 URL：

- `https://raw.githubusercontent.com/...`
- `https://cdn.jsdelivr.net/gh/...`
- `https://github.com/.../raw/...`
- `https://api.*`
- 以 `.txt`、`.json`、`.csv` 结尾的公开下载链接。

### 抽样验证

- 使用 `HEAD` 或小范围 `GET`。
- 限制响应体大小。
- 判断内容是否符合以下模式：
  - `ip:port`
  - `protocol://ip:port`
  - JSON 字符串数组、对象数组或 `data/items/results/proxies` 包裹数组。
  - 每行一个 URL 的源清单。

## 评分

候选源评分维度：

- 可访问性。
- 是否协议明确。
- 是否机器可读。
- 是否无需认证。
- 是否来自稳定宿主。
- 是否最近更新。
- 与已有源的重复率。

## 命令

```bash
plugproxy discover search -query "free proxy list socks5" -limit 10
plugproxy discover search -query "free proxy list socks5" -limit 10 -ai
plugproxy discover repo jhao104/proxy_pool -workers 16
plugproxy discover url https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/http.txt
plugproxy discover validate candidates.json -workers 128 -per-host-workers 4
plugproxy discover validate candidates.json -write-sources plugproxy.sources.candidates.json
```

AI 默认关闭。开启 AI 搜索时需要配置：

```bash
OPENAI_API_KEY=...
```

也可以使用 Responses-compatible Provider：

```bash
PLUGPROXY_AI_API_KEY=...
PLUGPROXY_AI_BASE_URL=https://example.com/v1
plugproxy discover search -query "proxy sources" -ai -ai-provider responses-compatible -ai-model gpt-5
```

## 数据流

```text
discover -> candidates -> validate source -> human review -> source config -> fetch -> check -> pool
```

当前主采集链路已经支持 `raw_text_url`、`html_text_url`、`br_text_url`、`json_url` 和 `api_url` 源配置。候选源经过人工确认后，可以写入 `plugproxy.sources.json`：

```json
{
  "name": "example-http",
  "type": "raw_text_url",
  "url": "https://example.com/http.txt",
  "protocol_hint": "http",
  "enabled": true
}
```

也可以让 `discover validate` 先导出一份默认禁用的候选源配置：

```bash
plugproxy discover validate candidates.json -write-sources plugproxy.sources.candidates.json
```

导出文件只包含验证通过且 `adapter_required=false` 的 `raw_text_url`、`html_text_url`、`br_text_url`、`json_url` 和 `api_url` 候选源，并统一写入 `"enabled": false`。推荐流程是先 review 这份文件，再手动合并到 `plugproxy.sources.json` 并开启可信源。

中文站点如果返回 `<br>` 分隔、简单 HTML 片段，或 `<td>IP</td><td>PORT</td>` 这种轻量表格单元格，可以使用 `html_text_url`：

```json
{
  "name": "example-cn-html",
  "type": "html_text_url",
  "url": "https://example.cn/free-proxy",
  "protocol_hint": "http",
  "enabled": true
}
```

JSON 候选源如果抽样验证能解析出代理，`discover validate` 会标记为 `adapter_required=false`，并输出 `recipe.parser=json_auto`。进入配置时可以使用自动解析，也可以补充轻量字段映射：

```json
{
  "name": "example-json",
  "type": "json_url",
  "url": "https://example.com/proxies.json",
  "protocol_hint": "http",
  "enabled": true,
  "json": {
    "items_path": "data",
    "host_field": "ip",
    "port_field": "port",
    "protocol_field": "protocol"
  }
}
```

公开 JSON API 可以用 `api_url`，第一版只增加请求头能力，不处理认证、分页、多请求聚合：

```json
{
  "name": "example-api",
  "type": "api_url",
  "url": "https://api.example.com/free-proxies",
  "protocol_hint": "http",
  "enabled": true,
  "headers": {
    "Accept": "application/json"
  }
}
```

## 安全边界

- 不绕过登录、验证码、付费墙。
- 不扫描无关网站。
- 不高频请求公共服务。
- 不默认启用新发现源。
- 不把未经检测的代理暴露给用户项目。

## AI Provider

发现模块只依赖 `AIProvider` 抽象，不和具体模型厂商强绑定。

第一版内置：

- `openai`：使用 OpenAI Responses API + `web_search`。
- `responses-compatible`：使用兼容 Responses API 的服务，通过 `-ai-base-url` 或 `PLUGPROXY_AI_BASE_URL` 配置。

后续可以新增：

- Anthropic。
- Gemini。
- OpenRouter。
- Ollama 或其他本地模型。

AI 的职责是搜索规划、结果理解和候选规则草案生成。网络请求、限速、抽样验证和候选源状态仍由 Go 代码控制。

## 第一版实现建议

第一版只做 GitHub 和 Raw URL：

- 使用 GitHub API 搜索仓库。
- 读取 README 和 `sources/`、`proxies/` 目录。
- 从文本中提取 URL。
- 对 URL 做抽样验证。
- 输出 `docs/proxy-sources.candidates.json` 或本地缓存文件。
- repo 文件扫描和候选源验证使用 worker pool；默认值保守，但可以通过 `-workers` 提高并发。

页面型源发现放到第二版。
