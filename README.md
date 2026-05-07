# plugproxy

plugproxy 是一个轻量的代理采集、检测和代理池服务。它可以从公开代理源抓取代理，做并发检测，给代理打健康状态，然后通过 CLI、HTTP API 或 Go SDK 给其他程序取用。

> 当前版本：v0.5.0 Core Foundation。
> 这一版重点收口基础能力：主配置、源管理、cache 维护、健康 API、检测调度、动态刷新和轻量观测。

## 适合谁

- 想快速搭一个本地代理池。
- 想把免费代理源采集、去重、检测、缓存这条链路自动化。
- 想通过 HTTP API 或 Go SDK 给自己的程序拿代理。
- 想研究代理源质量、检测调度、并发刷新和长期运行策略。

先说清楚：免费代理质量很不稳定，抓到很多不等于可用很多。plugproxy 的价值不是保证每个代理都活，而是把“抓取、检测、跳过、退避、缓存、取用”这套流程跑稳。

## 安装

从 GitHub Releases 下载对应系统的压缩包，解压后把 `plugproxy` 或 `plugproxy.exe` 放到 `PATH`。

验证安装：

```bash
plugproxy version
```

也可以直接从源码运行：

```bash
go run ./cmd/plugproxy version
```

## 五分钟跑起来

### 1. 初始化代理源

```bash
plugproxy init
```

这会生成 `plugproxy.sources.json`。如果你不生成这个文件，程序也会使用内置默认源，但显式文件更方便后续管理。

可选：初始化主配置文件。

```bash
plugproxy config init
plugproxy config validate
```

这会生成 `plugproxy.config.json`。配置优先级是：CLI 参数 > 主配置文件 > 默认值。

### 2. 抓取代理

```bash
plugproxy fetch -source-workers 32 -per-host-workers 4
```

结果会写入 `.plugproxy.cache.json`。输出里重点看：

- `successful_sources`：成功抓取的源数量
- `failed_sources`：失败源数量
- `fetched`：源里抓到的代理数量
- `added`：去重后加入池子的数量

### 3. 检测一批代理

先用小批量检测，别一上来全量打满：

```bash
plugproxy check -workers 64 -max-checks 300 -check-profile smart -target https://httpbin.org/ip -target-fallbacks https://api.ipify.org
```

输出里重点看：

- `scheduled`：本轮实际进入检测的代理数
- `healthy` / `degraded` / `dead`：检测后的状态
- `skipped_unsupported`：当前跳过的 unsupported 协议，例如 SOCKS4
- `error_types`：失败原因分布，例如 timeout、connection_error

### 4. 取一个可用代理

```bash
plugproxy get -strategy fastest -protocol http
```

`get` 默认不会返回 `dead`。选择顺序大致是：

```text
healthy -> degraded -> unchecked
```

如果你想看池子全量状态，用：

```bash
plugproxy list
plugproxy list -exclude-dead=true
plugproxy stats
```

### 5. 启动常驻服务

```bash
plugproxy run -addr 127.0.0.1:8899 -skip-check=false -refresh=true -max-checks 300
```

服务启动后可以访问：

```text
GET  http://127.0.0.1:8899/healthz
GET  http://127.0.0.1:8899/readyz
GET  http://127.0.0.1:8899/stats
GET  http://127.0.0.1:8899/proxy?strategy=fastest&protocol=http
GET  http://127.0.0.1:8899/metrics.json
POST http://127.0.0.1:8899/refresh
POST http://127.0.0.1:8899/refresh/cancel
```

观察运行状态：

```bash
plugproxy watch -api http://127.0.0.1:8899
```

## 常用命令速查

```bash
plugproxy version
plugproxy init
plugproxy doctor

plugproxy config init
plugproxy config validate
plugproxy config print

plugproxy fetch
plugproxy check -max-checks 300 -check-profile smart
plugproxy list
plugproxy get -strategy fastest -protocol http
plugproxy stats
plugproxy run -addr 127.0.0.1:8899 -skip-check=false
plugproxy watch

plugproxy sources list
plugproxy sources validate
plugproxy sources test
plugproxy sources add -name example -type raw_text_url -url https://example.com/proxies.txt -protocol-hint http
plugproxy sources enable example
plugproxy sources disable example
plugproxy sources remove example

plugproxy cache stats
plugproxy cache compact -max-entries 50000 -drop-dead-after 24h -drop-stale-after 168h
plugproxy cache repair

plugproxy discover search -query "free proxy list socks5" -limit 10
plugproxy discover validate candidates.json -write-sources plugproxy.sources.candidates.json
```

## 两个配置文件

plugproxy 有两个配置文件，职责分开：

- `plugproxy.sources.json`：代理源列表。
- `plugproxy.config.json`：运行参数，例如 server、cache、fetch、check、scheduler、refresh、logging。

初始化：

```bash
plugproxy init
plugproxy config init
```

验证：

```bash
plugproxy sources validate
plugproxy config validate
```

使用主配置启动：

```bash
plugproxy run -app-config plugproxy.config.json
```

## 支持的代理源

当前运行时支持：

- `raw_text_url`：解析 `ip:port` 和 `protocol://ip:port`
- `html_text_url`：从简单 HTML、`<br>` 文本、表格单元格里抽取代理
- `br_text_url`：兼容 `<br>` 分隔的页面或接口
- `json_url`：解析 JSON 字符串数组、对象数组、`data/items/results/proxies` 包装数组
- `api_url`：按 JSON API 处理，支持 headers

更完整说明见 [代理源清单](docs/proxy-sources.md) 和 [代理源发现爬虫设计](docs/source-discovery.md)。

## 健康状态

代理进入池子后有四种状态：

- `unchecked`：抓到了，但还没检测
- `healthy`：检测成功、分数较高、最近没有连续失败
- `degraded`：可能可用，但质量较低或检测结果一般
- `dead`：连续失败或健康分过低

默认取用策略：

- `plugproxy get` 和 `/proxy` 默认排除 `dead`
- `list` 和 `/proxies` 默认展示全量，方便排查
- 需要只看可用候选时使用 `-exclude-dead=true` 或 `?exclude_dead=true`

SOCKS5 已支持检测。SOCKS4 当前明确标记为 unsupported，smart profile 下会跳过，避免浪费检测 worker。

## 检测与调度

常用检测参数：

```bash
plugproxy check \
  -workers 64 \
  -max-checks 300 \
  -check-profile smart \
  -target https://httpbin.org/ip \
  -target-fallbacks https://api.ipify.org \
  -connect-timeout 5s \
  -response-header-timeout 5s
```

`smart` profile 会做：

- 未检测代理优先
- healthy/degraded/dead 分层复检
- dead 代理死亡退避
- SOCKS4 unsupported 跳过
- 有限预算下按协议和 source 公平抽样
- 可选 tail-biased，验证同源靠后代理是否更可能健康

## HTTP API

常用 API：

```text
GET  /healthz
GET  /readyz
GET  /stats
GET  /proxies
GET  /proxies?protocol=http&status=healthy&limit=100
GET  /proxy
GET  /proxy?strategy=fastest&protocol=http
GET  /sources
GET  /refresh
POST /refresh
POST /refresh/cancel
GET  /metrics.json
```

`/ui` 入口仍保留，但 v0.5.0 暂停继续开发管理面板，当前优先使用 CLI、HTTP API 和 `/metrics.json`。

## Go SDK

推荐通过 `pkg/client` 连接常驻服务：

```go
c := client.New("http://127.0.0.1:8899")
p, err := c.GetProxy(ctx, client.GetProxyOptions{
	Strategy: "fastest",
	Protocol: model.ProtocolHTTP,
})
```

也可以用 `pkg/plugproxy` 在业务进程内嵌入服务。见 [Go SDK 接入](docs/sdk.md)。

## 项目结构

```text
cmd/plugproxy/       CLI 入口
internal/app/        应用编排
internal/cache/      代理缓存
internal/checker/    代理检测
internal/config/     配置和默认源
internal/discover/   代理源发现、验证和 AI Provider
internal/fetcher/    并发代理源采集
internal/pool/       代理池接口与内存实现
internal/server/     HTTP API
internal/source/     代理源接口与实现
pkg/client/          HTTP Client SDK
pkg/model/           公开代理数据模型
pkg/plugproxy/       嵌入式 SDK
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
