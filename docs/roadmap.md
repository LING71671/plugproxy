# 开发路线图

## 第一阶段：可运行骨架

- 初始化 Go module。
- 提供 CLI 入口。
- 定义代理模型、代理源、检测器、代理池基础接口。
- 实现内存代理池。
- 实现轻量 HTTP API。

## 第二阶段：真实代理源

- 支持纯文本代理列表。
- 支持 GitHub Raw 代理源。
- 支持 JSON 代理源。
- 支持公开 JSON API 源和请求头配置。
- 支持从配置文件加载代理源。
- 增加代理去重和基础格式归一化。
- 增加 `discover` 体系，支持 GitHub、Raw URL 和可选 AI 搜索发现候选代理源。
- 打通默认 Raw/API TXT 源到 `fetch/list/check/run` 主链路。
- 增加源级采集报告、代理缓存和所有源失败时的缓存回退。

## 第三阶段：检测与评分

- 支持 HTTP 和 HTTPS 代理检测。
- 支持 SOCKS4 和 SOCKS5 代理检测。
- 支持可配置检测目标。
- 增加延迟、失败次数、成功率、最后可用时间等评分字段。
- 增加 `any`、`fastest`、`healthy` 等选择策略。
- 增加复杂健康评分和 `healthy/degraded/dead/unchecked` 状态分级。
- 增加持久健康池，支持检测结果跨 CLI 执行和服务重启复用。

## 第四阶段：接入能力

- 稳定 HTTP API。
- 提供 Go SDK。
- 提供 CLI JSON 输出，方便脚本接入。
- 提供本地代理获取命令，例如 `plugproxy get`。
- 增加 `plugproxy init` 和 `plugproxy doctor`，降低上手和排错成本。
- 增加后台自动刷新和异步 `POST /refresh` 触发接口。
- 增加 HTTP Client SDK、嵌入式 SDK、`/stats` 和 `plugproxy stats`。
- 增加 v0.2.0 tag release、跨平台二进制和 checksums。
- 增加 v0.2.1 稳定性版本，收口 smart check scheduler、source 冷却、host 限流、错误分类、refresh 可观测和 discover 配置导出。
- 增加 v0.3.0 工程基础版本，收口 atomic cache write、HTTP Transport 配置、refresh cancel、优雅关闭和日志配置。
- 增加 v0.4.0 观测与控制台版本，提供 `/metrics.json` 和嵌入式 Svelte Console。

## 第五阶段：管理与可视化

- 增加可选轻量持久化。
- 增加嵌入式管理面板。
- 支持代理源启停、代理状态查看、手动刷新和检测。

## 横向主题：并发能力

- 已增加 `check-ttl` 和 `max-checks`，减少无意义全量检测。
- 已增加 smart check profile，支持分层复检、死亡退避、协议公平和 unsupported 跳过。
- 已增加源级失败统计、冷却和 host 级并发限制。
- 已增强 refresh 状态，暴露阶段、进度、耗时和取消状态。
- 已对源抓取和检测错误分类，区分超时、连接错误、协议不支持和响应异常。
- 已支持 `discover validate -write-sources` 导出待人工确认的源配置。
- 后续继续推进 source 半开试探、全局连接预算、SOCKS4 真检测、metrics 和 sources 管理命令。

详细设计见 [并发能力设计备忘](concurrency.md)。
