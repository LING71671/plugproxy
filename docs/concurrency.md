# 并发能力设计备忘

## 目标

plugproxy 的并发能力不只追求更大的 worker 数，而是追求在免费源波动、代理高失败率和本机资源有限的情况下，稳定地完成采集、验证、检测和刷新。

核心目标：

- 让慢源、坏源、坏代理尽快退出，不拖垮整轮任务。
- 在高并发下保持可取消、可观测、可限流。
- 减少无意义工作，优先检测最可能有价值的代理。
- 刷新不能依赖写死的固定周期作为核心策略，必须逐步演进到基于反馈的动态调控。
- 保持轻量，优先使用标准库和小型内部抽象。

## 当前基础

- `fetch -source-workers` 控制源级采集并发。
- `check -workers` 控制代理检测并发。
- `discover validate -workers` 控制候选源验证并发。
- `run` 后台刷新串行执行，避免多轮 refresh 重入。
- 源请求已有 timeout、body_limit 和 context。
- 采集失败会被源级隔离，所有源失败时可以回退缓存。

## 总体思路

并发增强分四个方向：

- 控制入口：限制 worker、队列、连接、每源请求频率。
- 降低任务量：只检测新增、过期、可疑或高价值代理。
- 更快失败：细分超时、熔断、失败退避、错误分类。
- 观测反馈：记录每阶段耗时、队列长度、成功率、取消状态，用数据调默认值。

当前 `run/refresh` 已采用第一版源结果级流水线：

```text
cache preload -> schedule checks
source workers -> source result -> dedupe -> schedule checks -> check workers -> pool -> save cache
```

第一版流水线以 source result 为边界：一个源内部仍由 adapter 完整抓取和解析，源返回一批代理后立刻进入去重和检测调度；不做单个响应体内部的流式解析。

## 采集源并发

可做事项：

- 保留全局 `source-workers`，避免一次性打满所有源。
- 增加每源连续失败计数，连续失败后进入短期熔断。
- 增加源级冷却时间，例如连续失败 3 次后 15 分钟内跳过。
- 给源报告增加最近成功时间、最近失败时间、连续失败次数。
- 对同一 host 做并发限制，避免多个源实际打到同一个公共服务。
- 对大型源保持 `body_limit`，后续 raw text 可考虑流式解析。
- 对 JSON/API 源保留 `headers`，但不做认证、登录或分页聚合。

建议配置草案：

```json
{
  "source_workers": 32,
  "per_host_workers": 4,
  "source_failure_threshold": 3,
  "source_cooldown": "15m"
}
```

## 候选源验证并发

`discover validate` 的并发目标是快速排除不可用候选源，而不是把远端服务打满。

可做事项：

- 复用采集源的 host 限流逻辑。
- 验证阶段只读取 sample，不读取完整大文件。
- 对 JSON/API 候选源使用同一个 JSON parser 做抽样解析。
- 记录验证结果里的 `count`、`duration_ms`、`content_type`、`status_code`。
- 对不可访问、非代理内容、需要认证、HTML 页面型分别标注原因。
- 对候选源按 host 分散执行，避免同一 host 队列挤在一起。

## 检测并发

代理检测是最容易消耗 socket、CPU 和时间的阶段。提升思路优先是减少检测量，其次才是增加 worker。

可做事项：

- 增量检测：优先检测新增代理。
- 过期检测：只复检超过 TTL 的代理。
- 分层检测：`unchecked`、`degraded`、`dead`、`healthy` 使用不同复检频率。
- 健康代理低频复检，死亡代理指数退避。
- 支持 `-max-checks`，限制单轮最多检测数量。
- 支持 `-check-ttl`，跳过最近刚检测过的代理。
- 按协议分配并发，例如 HTTP/SOCKS5 使用不同 worker 上限。
- 增加全局 socket/连接令牌，避免检测 worker 过多导致本机资源耗尽。
- 错误分类：连接拒绝、超时、协议不支持、DNS 错误、响应不匹配分别计数。

建议 CLI 草案：

```bash
plugproxy check -workers 128 -max-checks 5000 -check-ttl 30m
plugproxy run -workers 128 -max-checks 5000 -check-ttl 30m
```

## 调度策略

当前第一版检测调度器使用 TTL + 稳定排序，不引入第三方依赖，也不做复杂评分。

已落地规则：

- `CheckCount == 0` 或 `LastCheckedAt` 为空的代理优先进入本轮检测。
- `-check-ttl > 0` 时，最近检测过的代理会跳过并计入 `skipped_recent`。
- 候选代理按 `unchecked > healthy > degraded > dead` 稳定排序。
- 同状态下更久未检测的代理优先，再按 `health_score` 高者优先。
- `-max-checks > 0` 时只检测排序后的前 N 个，其余计入 `skipped_limit`。

后续可以从简单排序演进到优先级队列，但第一版不需要复杂调度器。

推荐优先级：

1. 新采集到且从未检测的代理。
2. 曾经健康但检测结果过期的代理。
3. 最近失败但未达到死亡阈值的代理。
4. 长时间未复检的死亡代理。
5. 本轮重复出现次数较高的代理。

可以先用排序后的 slice 实现，不急着引入 heap。排序字段可以是健康状态、最近检测时间、来源权重、历史成功率。

后续算法方向：

- Score ranking：按健康分、来源信誉、重复出现次数和最近失败惩罚打分。
- Weighted round robin：把新增/健康、普通过期、死亡退避代理分通道按比例检测。
- EWMA：平滑源成功率、响应时间、超时率和代理检测延迟。
- AIMD：根据超时率、错误率和 429/5xx 信号动态调整并发。
- Circuit breaker：对连续失败的源短期熔断，冷却后半开试探。

## HTTP Transport 与超时

高并发检测时需要明确超时层级：

- 整轮任务 context：由 CLI 或 refresh 控制。
- 单代理检测 timeout：由 `-timeout` 控制。
- 连接超时、TLS 握手超时、响应头超时：由 Transport 控制。
- 空闲连接回收：避免长期运行服务积累连接。

HTTP/HTTPS 检测可继续使用标准库 `http.Transport`，后续考虑配置：

- `MaxIdleConns`
- `MaxIdleConnsPerHost`
- `IdleConnTimeout`
- `TLSHandshakeTimeout`
- `ResponseHeaderTimeout`

## 代理池与锁竞争

并发提高后，内存池可能遇到锁竞争。

可做事项：

- 检测结果先在 worker 外聚合，再批量写入 pool。
- `List` 返回快照，避免调用方长时间持锁。
- 后续如果代理数量很大，可以考虑按 protocol 或 hash 分片锁。
- 保持写缓存在整轮结束后执行，不在每个代理检测后写文件。

## 缓存与持久化

缓存策略影响并发体验，尤其是 `run` 长期运行时。

可做事项：

- cache 写入使用临时文件加 rename，避免中途失败留下坏文件。
- 大缓存写入只在状态有变化时执行。
- refresh 中 fetch/check 完成后统一写一次。
- 后续可增加轻量索引字段，例如 `last_seen_at`、`seen_count`。
- 长期可以评估 BoltDB/SQLite，但 V1.0 前优先保持 JSON 文件。

## 后台刷新

当前后台刷新使用源结果级流水线，源抓取和代理检测可以并行运行。后续重点是从固定间隔刷新演进到动态调控，固定 `refresh-interval` 只能作为安全下限或兜底节拍，不能作为唯一决策依据。

可做事项：

- refresh status 增加阶段：`fetching`、`checking`、`saving`、`idle`。
- 增加当前进度：已处理源数量、已检测代理数量、总任务数。
- `POST /refresh` 返回是否已排队、是否因运行中被跳过。
- 支持取消当前 refresh，但需要清晰定义半成品是否写缓存。
- 自动刷新只检测新增和过期代理，避免每 5 分钟全量检测。

动态刷新调控的输入信号：

- 池内可用代理数量低于水位线。
- 健康代理占比下降。
- `get/proxy` 出现空池或可用代理不足。
- 新源发现、源配置变更或手动触发 refresh。
- 源连续失败、HTTP 429/5xx、超时率升高。
- 上一轮 refresh 的耗时、抓取数量、检测成功率和跳过比例。

动态刷新调控的输出动作：

- 提前触发 refresh，而不是等固定周期。
- 推迟下一轮 refresh，并加入随机抖动，避免周期性请求撞车。
- 降低某个源或 host 的抓取频率。
- 降低检测并发或缩小单轮 `max-checks`。
- 对连续失败源进入冷却，冷却后只放小流量试探。
- 健康池充足时延长刷新间隔，只做低频保活。

第一版动态刷新不需要复杂控制器，可以采用“事件触发 + 退避兜底”：

```text
needs_refresh = healthy_count < min_healthy
             || healthy_ratio < min_healthy_ratio
             || unchecked_count > unchecked_threshold
             || manual_trigger

next_delay = base_interval * failure_backoff * load_factor + jitter
```

其中 `base_interval` 只是基础节拍，`failure_backoff` 来自源失败率和 refresh 失败次数，`load_factor` 来自健康池是否充足，`jitter` 用于打散固定周期。

Dynamic Refresh V1 已采用“水位线 + 兜底 + 抖动”：

- `refresh-interval` 映射为 `base_interval`，只作为基础间隔。
- `refresh-min-interval` 防止低水位时连续刷新过密。
- `refresh-max-interval` 作为最大兜底等待时间。
- `refresh-jitter` 给下一次等待增加随机抖动。
- `min-healthy` 和 `min-healthy-ratio` 控制健康池水位线。
- `unchecked-threshold` 控制未检测代理积压触发。
- `refresh-failure-backoff` 在上一轮有失败源或 cache 错误时放大下一次等待。
- `GET /refresh` 暴露 `next_at`、`last_reason` 和当前 `policy`。

V1 不做 source 级单独频率控制，不做 EWMA/AIMD，也不改变流水线内部并发。

参考算法：

- Exponential backoff + jitter：失败后拉长间隔并增加随机抖动，避免固定周期和重试风暴。
- Token bucket：把 refresh、源请求和检测任务放进预算内执行，预算耗尽时延后而不是硬冲。
- Circuit breaker：连续失败后打开熔断，冷却后半开试探，适合源级调控。
- AIMD：成功时缓慢增加频率或并发，遇到超时、429、5xx 时快速降低。
- EWMA：平滑源响应时间、成功率、健康率和超时率，避免单次波动导致过度反应。

## 观测指标

建议先以 JSON 报告暴露，不急着接 Prometheus。

源级指标：

- `duration_ms`
- `count`
- `status`
- `error`
- `consecutive_failures`
- `last_success_at`
- `last_failure_at`

检测级指标：

- `checked`
- `skipped_recent`
- `skipped_limit`
- `healthy`
- `degraded`
- `dead`
- `unsupported`
- `timeout`
- `connection_error`
- `duration_ms`

刷新级指标：

- `phase`
- `started_at`
- `finished_at`
- `duration_ms`
- `cancelled`
- `fetch_report`
- `check_stats`

## 默认值建议

默认值需要保守，用户可以手动调高。

- `source-workers`: 32
- `discover validate workers`: 128
- `check workers`: 128
- `per-host workers`: 4
- `source timeout`: 12s
- `check timeout`: 8s
- `check ttl`: 30m
- `max checks`: 0，表示不限制
- `source cooldown`: 15m

## 分阶段实施

### P0：低风险收益

- 已增加 `check-ttl`，跳过最近检测过的代理。
- 已增加 `max-checks`，限制单轮检测规模。
- 已增加检测调度统计：`scheduled`、`skipped_recent`、`skipped_limit`。
- refresh/status 增加阶段和进度。
- doctor/source report 增加源级耗时与错误汇总。

### P1：稳定高并发

- 增加源级连续失败计数和冷却。
- 增加 host 级并发限制。
- 检测结果错误分类。
- HTTP Transport 参数集中配置。
- cache 写入改为临时文件加 rename。

### P2：调度优化

- 按健康状态和检测过期时间选择检测任务。
- 健康代理低频复检，死亡代理退避复检。
- 按协议设置检测并发。
- 引入轻量优先级队列或排序调度。

### P3：大规模运行

- raw text 流式解析。
- pool 分片锁或批量更新接口。
- 可选持久化后端。
- 更完整的运行指标和导出接口。

## 风险

- worker 数过高可能耗尽本机端口、文件描述符或网络带宽。
- 免费代理大量超时，过长 timeout 会让队列堆积。
- 对公共源过高并发会造成不友好访问，也容易被封禁。
- 过度复杂的调度器会降低可维护性。
- 缓存频繁写入会放大 IO，甚至损坏未完成写入。

## 判断标准

并发增强完成后，应能回答这些问题：

- 一轮 fetch/check 花了多久？
- 时间主要耗在哪个源、哪个协议、哪类错误？
- 有多少代理因为 TTL 或上限被跳过？
- 当前 refresh 处于什么阶段，能否取消？
- 调高 worker 后吞吐是否真的提升，还是只是超时更多？

## 参考

- AWS Architecture Blog: Exponential Backoff and Jitter
- Amazon Builders' Library: Timeouts, retries, and backoff with jitter
- Google SRE Book: Handling Overload / Client-side throttling
- RFC 2914: Congestion Control Principles
- Netflix concurrency-limits: adaptive concurrency limit algorithms
