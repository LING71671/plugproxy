# plugproxy

plugproxy 是一个使用 Go 编写的轻量级代理采集、检测、代理池管理和接入工具。

> 当前状态：v0.4.0 可用预览版。

## 目标

- 从多种免费代理源采集代理。
- 并发检测代理，并保留有价值的健康状态与元数据。
- 通过 CLI、HTTP API 和 Go SDK 让任何项目都能轻松接入代理池。
- 支持 GitHub、Web、Raw URL 和可选 AI 搜索发现候选代理源。
- 以 worker pool 支持高并发抓取、验证和检测。
- 保持轻量：单个 Go 二进制，无强制外部服务依赖。

## 安装

从 GitHub Releases 下载适合系统的压缩包，解压后把 `plugproxy` 或 `plugproxy.exe` 放到 `PATH` 中。

```bash
plugproxy version
plugproxy init
plugproxy doctor
```

也可以直接从源码运行：

```bash
go run ./cmd/plugproxy version
go run ./cmd/plugproxy init
go run ./cmd/plugproxy doctor
```

## 快速开始

```bash
plugproxy init
plugproxy doctor
plugproxy fetch -source-workers 32 -per-host-workers 4 -cache .plugproxy.cache.json
plugproxy check -source-workers 32 -cache .plugproxy.cache.json -workers 128 -protocol http -target https://httpbin.org/ip -timeout 8s -connect-timeout 5s -max-checks 300 -check-profile smart
plugproxy get -cache .plugproxy.cache.json -strategy fastest -protocol http -healthy=true
plugproxy run -source-workers 32 -per-host-workers 4 -cache .plugproxy.cache.json -addr 127.0.0.1:8899 -skip-check=false -refresh=true -refresh-interval 5m -refresh-min-interval 30s -refresh-max-interval 30m -shutdown-timeout 10s -log-level info -max-checks 300
```

Go 项目接入见 [Go SDK 接入](docs/sdk.md)。

## 架构

```mermaid
flowchart LR
    subgraph Discovery["代理源发现"]
        GH["GitHub API\nREADME / sources / fetcher"]
        WEB["Web Search\n可选 AI Provider"]
        RAW["Raw URL\nTXT / JSON / HTML"]
        AI["AI Analyst\n搜索规划 / 结果理解 / 规则草案"]
    end

    subgraph Candidates["候选源层"]
        CS["CandidateSource\n候选待审"]
        VR["Validate Workers\n抽样验证 / 去重 / 打分"]
        SR["SourceRecipe\n解析规则草案"]
    end

    subgraph Fetch["采集与检测"]
        SRC["Source Adapter\nRaw / JSON / HTML / API"]
        FW["Fetch Workers\n高并发采集"]
        CK["Check Workers\n高并发检测"]
        POOL["Proxy Pool\n健康评分 / 策略选择"]
    end

    subgraph Access["接入层"]
        CLI["CLI"]
        API["HTTP API"]
        SDK["Go SDK"]
        UI["轻量管理面板\n后续"]
    end

    GH --> CS
    WEB --> AI --> CS
    RAW --> CS
    CS --> VR --> SR
    SR --> SRC
    SRC --> FW --> CK --> POOL
    POOL --> CLI
    POOL --> API
    POOL --> SDK
    POOL --> UI
```

## 逻辑链

```text
discover -> candidates -> validate -> review -> source config -> fetch -> check -> pool -> CLI / HTTP API / SDK
```

plugproxy 的核心原则是“发现候选源”和“使用可用代理”分离。AI、GitHub 搜索和页面分析只进入候选源队列；代理必须经过抽样验证、采集、检测和健康评分后，才会进入代理池。

## 扩展点

- `AIProvider`：适配 OpenAI、Responses-compatible 服务，以及后续 Anthropic、Gemini、OpenRouter、Ollama 等。
- `Source`：适配 Raw TXT、JSON、HTML 表格、公开 API 和项目源码引用的页面型源。
- `Checker`：扩展 HTTP、HTTPS、SOCKS4、SOCKS5 和多目标检测。
- `Pool`：扩展内存池、持久化池、健康评分和选择策略。
- `Access`：扩展 CLI、HTTP API、Go SDK 和后续嵌入式前端管理面板。

## 当前命令

```bash
go run ./cmd/plugproxy version
go run ./cmd/plugproxy init
go run ./cmd/plugproxy doctor
go run ./cmd/plugproxy fetch -source-workers 32 -per-host-workers 4 -cache .plugproxy.cache.json
go run ./cmd/plugproxy list -source-workers 32 -cache .plugproxy.cache.json
go run ./cmd/plugproxy get -source-workers 32 -cache .plugproxy.cache.json -strategy fastest -protocol http -healthy=true
go run ./cmd/plugproxy stats -cache .plugproxy.cache.json
go run ./cmd/plugproxy check -source-workers 32 -per-host-workers 4 -cache .plugproxy.cache.json -workers 128 -protocol http -target https://httpbin.org/ip -timeout 8s -connect-timeout 5s -response-header-timeout 5s -max-checks 300 -check-profile smart
go run ./cmd/plugproxy run -source-workers 32 -per-host-workers 4 -source-cooldown 15m -cache .plugproxy.cache.json -addr 127.0.0.1:8899 -skip-check=false -refresh=true -refresh-interval 5m -refresh-min-interval 30s -refresh-max-interval 30m -shutdown-timeout 10s -log-level info -max-checks 300 -check-profile smart
go run ./cmd/plugproxy discover repo jhao104/proxy_pool -workers 32
go run ./cmd/plugproxy discover url https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/http.txt
go run ./cmd/plugproxy discover search -query "free proxy list socks5" -limit 10 -workers 32
go run ./cmd/plugproxy discover validate candidates.json -workers 128 -per-host-workers 4 -write-sources plugproxy.sources.candidates.json
```

运行后可用的 HTTP API：

```text
GET /health
GET /ui
GET /metrics.json
GET /sources
GET /refresh
POST /refresh
GET /stats
GET /proxies
GET /proxies?protocol=http&status=healthy&limit=100&offset=0
GET /proxy
GET /proxy?protocol=http
GET /proxy?strategy=fastest
GET /proxy?strategy=fastest&protocol=http&healthy=true
```

## 健康评分

`check` 会按协议检测代理并更新内存代理池中的健康状态：

- HTTP/HTTPS：使用标准库 HTTP Transport 检测。
- SOCKS5：使用 Go 官方扩展包 `golang.org/x/net/proxy` 检测。
- SOCKS4：第一版明确标记为 unsupported，不参与健康代理判断。

健康字段包括 `health_score`、`health_status`、`check_count`、`consecutive_failures`、`last_success_at`、`last_failure_at` 和 `last_error`。

状态分级：

- `unchecked`：尚未检测。
- `healthy`：分数较高且最近一次检测成功。
- `degraded`：可疑或分数中等。
- `dead`：连续失败或分数过低。

健康状态会写入 `.plugproxy.cache.json`，独立执行一次 `check` 后，下一次 `get/list/run` 会复用历史健康评分。

`check` 和 `run` 支持检测调度：`-max-checks` 可限制单轮检测数量，`-check-profile smart` 会按健康状态分层复检、跳过 SOCKS4 unsupported、对死亡代理退避，并在有限预算下按协议和 source 公平抽样。`-tail-biased` 会在每个源内部优先抽取靠后的代理，用于验证“尾部是否更健康”的假设。`check` 默认 `full` 保持兼容；`run/refresh` 默认 `smart`，适合长期服务模式。高并发检测还可以通过 `-connect-timeout`、`-tls-handshake-timeout`、`-response-header-timeout`、`-idle-conn-timeout`、`-max-idle-conns` 和 `-max-idle-conns-per-host` 控制 HTTP Transport 行为。

取用代理时按 `healthy -> degraded -> unchecked` 排序，`dead` 默认不作为可用代理返回：`plugproxy get` 和 HTTP `/proxy` 默认启用 `exclude_dead`。`list` 仍默认展示全量，方便诊断；需要列出可用候选时可加 `-exclude-dead=true` 或 `/proxies?exclude_dead=true`。

## 代理源配置

默认会读取 `plugproxy.sources.json`。如果文件不存在，plugproxy 会启用内置的第一批高优先级 Raw/API TXT 源。

配置示例见 [plugproxy.sources.example.json](plugproxy.sources.example.json)：

```json
{
  "sources": [
    {
      "name": "proxyscrape-http",
      "type": "raw_text_url",
      "url": "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=http&proxy_format=ipport&format=text",
      "protocol_hint": "http",
      "enabled": true,
      "timeout": "12s",
      "body_limit": 2097152
    }
  ]
}
```

当前运行时支持 `raw_text_url`、`html_text_url`、`br_text_url`、`json_url` 和 `api_url`：

- `raw_text_url`：解析 `ip:port` 和 `protocol://ip:port`。
- `html_text_url` / `br_text_url`：从简单 HTML、`<br>` 分隔文本、页面片段或 `<td>IP</td><td>PORT</td>` 表格单元格中抽取代理，用于 89IP、快代理这类中文免费代理页面；不执行 JS，不绕过验证码或登录。
- `json_url`：解析字符串数组、对象数组，以及 `data/items/results/proxies` 等根对象数组。
- `api_url`：第一版按 JSON API 处理，可通过 `headers` 设置 `Accept`、`User-Agent` 等公开 API 请求头。

JSON/API 字段映射可用 `json.items_path`、`json.proxy_field`、`json.host_field`、`json.port_field`、`json.protocol_field` 做轻量覆盖；`items_path` 只支持单层 key。更完整的源接入说明见 [代理源清单](docs/proxy-sources.md)。

## 采集缓存

`fetch/list/get/check/run` 默认会把成功采集到的代理写入 `.plugproxy.cache.json`。当一轮采集所有源都失败时，会自动回退读取缓存，避免免费源短暂超时导致代理池为空。

`fetch` 会输出本轮源级采集报告，包括成功源、失败源、冷却跳过源、去重数量、错误分类、缓存是否被复用和每个源耗时。连续失败源会进入短期冷却，同一 host 的源请求也会受 `-per-host-workers` 限制。HTTP API 的 `GET /sources` 会返回最近一次采集报告。

缓存也会保留检测后的健康字段。缓存中的代理与新采集代理按 `protocol://address` 合并；新代理首次出现时初始化为 `unchecked` 和 50 分，已存在代理再次出现时保留历史健康评分。

## 自动刷新

`run` 默认开启动态刷新控制器。`-refresh-interval` 是基础间隔，不是写死节拍；控制器会根据健康代理水位、unchecked 积压、上一轮失败情况和抖动计算下一次刷新。刷新会在源返回后尽快去重、调度检测并入池，不必等待所有源都抓取完成；最终仍只写一次 cache。可用 `-refresh=false` 关闭，或用 `-refresh-min-interval`、`-refresh-max-interval`、`-min-healthy` 等参数调整策略。

`GET /refresh` 会显示当前 `phase`、进度、跳过原因、下一次刷新时间和最近一次 pipeline 报告；重复 `POST /refresh` 不会重入正在运行的 refresh。`POST /refresh/cancel` 可以取消当前 refresh，未运行时会返回 `skipped/not_running`。

## 管理控制台

`run` 会嵌入一个轻量 Svelte 管理控制台：

```text
GET /ui           打开控制台
GET /metrics.json 获取控制台使用的观测快照
```

控制台以 NOC/Grafana/Bloomberg 风格展示代理池、采集源、检测调度、刷新状态和运行时资源。关键数字会从旧值平滑过渡到新值；pipeline 动效来自 `/metrics.json` 的真实 delta，不播放无数据假动画。前端构建产物已嵌入 Go 二进制，运行时不需要 Node。

刷新任务串行执行，上一轮未结束时会跳过新一轮。HTTP API 提供：

```text
POST /refresh  异步触发一次刷新
GET /refresh   查看最近一次刷新状态
POST /refresh/cancel  取消当前刷新
```

`run` 支持 `-shutdown-timeout` 做优雅关闭，收到 `Ctrl+C` 或 SIGTERM 时会取消后台刷新并等待 HTTP server 退出。日志可用 `-log-level` 和 `-log-format` 调整，日志写入 stderr，命令 JSON 输出仍写 stdout。

## Go SDK

推荐外部项目通过 `pkg/client` 连接常驻 plugproxy 服务：

```go
c := client.New("http://127.0.0.1:8899")
p, err := c.GetProxy(ctx, client.GetProxyOptions{
	Strategy: "fastest",
	Protocol: model.ProtocolHTTP,
	Healthy:  true,
})
```

需要业务进程自带代理池时，可以使用 `pkg/plugproxy` 启动嵌入式服务。详细示例见 [Go SDK 接入](docs/sdk.md)。

## 项目结构

```text
cmd/plugproxy/       CLI 入口
internal/app/        应用编排
internal/cache/      代理缓存
internal/checker/    代理检测
internal/config/     代理源配置和默认源
internal/fetcher/    并发代理源采集
internal/pool/       代理池接口与内存实现
internal/server/     轻量 HTTP API
internal/source/     代理源接口与实现
internal/discover/   代理源发现、验证和 AI Provider
pkg/model/           公开代理数据模型
docs/                项目文档
```

## 文档

- [项目约定](docs/project-conventions.md)
- [GitHub Actions CI/CD](docs/ci-cd.md)
- [发布流程](docs/release.md)
- [代理源清单](docs/proxy-sources.md)
- [代理源发现爬虫设计](docs/source-discovery.md)
- [并发能力设计备忘](docs/concurrency.md)
- [Go SDK 接入](docs/sdk.md)
- [开发路线图](docs/roadmap.md)
