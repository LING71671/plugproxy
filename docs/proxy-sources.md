# 代理源清单

> 更新时间：2026-05-07。免费代理源变化很快，所有源都必须经过 plugproxy 自己的检测、去重、评分和隔离流程。

## 收集原则

- 优先选择可程序化接入的源：Raw TXT、JSON、CSV、公开 API。
- 优先选择协议明确的源：HTTP、HTTPS、SOCKS4、SOCKS5 分文件或带字段。
- 默认低频抓取，尊重源站限制，避免对公共源造成压力。
- 免费代理不可信，不能直接进入可用池，必须先进入候选池并检测。
- 页面型源先降级处理，优先做稳定 Raw/API 源。

## 第一批优先接入

### ProxyScrape

- 类型：公开 API
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT、JSON、CSV
- 优先级：高
- 入口：
  - `https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&proxy_format=protocolipport&format=text`
- 备注：官方页面提供 API URL，并说明免费列表持续更新和检测。

### Proxifly Free Proxy List

- 类型：GitHub/CDN Raw
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT、JSON、CSV
- 优先级：高
- 入口：
  - `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/all/data.txt`
  - `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/http/data.txt`
  - `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/https/data.txt`
  - `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks4/data.txt`
  - `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks5/data.txt`
- 备注：README 标注每 5 分钟更新，支持 `.json`、`.txt`、`.csv`。

### dpangestuw/Free-Proxy

- 类型：GitHub Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：高
- 入口：
  - `https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/All_proxies.txt`
  - `https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/http_proxies.txt`
  - `https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/socks4_proxies.txt`
  - `https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/socks5_proxies.txt`
- 备注：README 标注每 5 分钟更新，已按协议拆分。

### TheSpeedX/PROXY-List

- 类型：GitHub Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：高
- 入口：
  - `https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt`
  - `https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt`
  - `https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks5.txt`
- 备注：项目历史较久，适合作为基础候选源。

### ProxyScraper/ProxyScraper

- 类型：GitHub Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：高
- 入口：
  - `https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/http.txt`
  - `https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/socks4.txt`
  - `https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/socks5.txt`
- 备注：项目页面标注每 30 分钟更新。

### OpenProxyList

- 类型：公开 Raw API
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：高
- 入口：
  - `https://api.openproxylist.xyz/http.txt`
  - `https://api.openproxylist.xyz/https.txt`
  - `https://api.openproxylist.xyz/socks4.txt`
  - `https://api.openproxylist.xyz/socks5.txt`
- 备注：页面标注每 10 分钟更新。

### proxy-list.download

- 类型：公开 API
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：中
- 入口：
  - `https://www.proxy-list.download/api/v1/get?type=http`
  - `https://www.proxy-list.download/api/v1/get?type=https`
  - `https://www.proxy-list.download/api/v1/get?type=socks4`
  - `https://www.proxy-list.download/api/v1/get?type=socks5`
- 备注：API 文档明确 `type` 参数。

## 第二批增强源

### Firmfox/Proxify

- 类型：GitHub Raw
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：中
- 入口：
  - `https://raw.githubusercontent.com/Firmfox/Proxify/main/proxies/http.txt`
  - `https://raw.githubusercontent.com/Firmfox/Proxify/main/proxies/https.txt`
  - `https://raw.githubusercontent.com/Firmfox/Proxify/main/proxies/socks4.txt`
  - `https://raw.githubusercontent.com/Firmfox/Proxify/main/proxies/socks5.txt`
- 备注：项目 README 说明会从公开源收集并维护多协议代理。

### LoneKingCode/free-proxy-db

- 类型：GitHub Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT、JSON
- 优先级：中
- 入口：
  - `https://raw.githubusercontent.com/LoneKingCode/free-proxy-db/main/proxies/all.txt`
  - `https://raw.githubusercontent.com/LoneKingCode/free-proxy-db/main/proxies/http.txt`
  - `https://raw.githubusercontent.com/LoneKingCode/free-proxy-db/main/proxies/socks4.txt`
  - `https://raw.githubusercontent.com/LoneKingCode/free-proxy-db/main/proxies/socks5.txt`
  - `https://raw.githubusercontent.com/LoneKingCode/free-proxy-db/main/proxies/all.json`
- 备注：包含 TXT 和 JSON 两种格式，适合验证通用 adapter。

### monosans/proxy-list

- 类型：GitHub Raw JSON
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：JSON
- 优先级：中
- 入口：
  - `https://raw.githubusercontent.com/monosans/proxy-list/main/proxies.json`
  - `https://raw.githubusercontent.com/monosans/proxy-list/main/proxies_pretty.json`
- 备注：包含地理信息，适合做 JSON adapter 和元数据映射。

### IPLocate free-proxy-list

- 类型：GitHub/CDN Raw
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：高
- 入口：
  - `https://cdn.jsdelivr.net/gh/iplocate/free-proxy-list@main/protocols/http.txt`
  - `https://cdn.jsdelivr.net/gh/iplocate/free-proxy-list@main/protocols/https.txt`
  - `https://cdn.jsdelivr.net/gh/iplocate/free-proxy-list@main/protocols/socks4.txt`
  - `https://cdn.jsdelivr.net/gh/iplocate/free-proxy-list@main/protocols/socks5.txt`
- 备注：README 标注每 30 分钟验证更新；本地 HTTP 抽样检测中健康贡献较稳定，适合作为候选源。

### Skillter/ProxyGather

- 类型：GitHub/CDN Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：中
- 入口：
  - `https://cdn.jsdelivr.net/gh/Skillter/ProxyGather@master/proxies/working-proxies-all.txt`
  - `https://cdn.jsdelivr.net/gh/Skillter/ProxyGather@master/proxies/working-proxies-http.txt`
  - `https://cdn.jsdelivr.net/gh/Skillter/ProxyGather@master/proxies/working-proxies-socks4.txt`
  - `https://cdn.jsdelivr.net/gh/Skillter/ProxyGather@master/proxies/working-proxies-socks5.txt`
- 备注：项目说明通过 GitHub Actions 自动抓取和检测；本地抽样中连接成功数较高，但当前 healthy 会波动，建议默认禁用、人工开启。

### joy-deploy/free-proxy-list

- 类型：GitHub Raw
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：TXT、JSON
- 优先级：中
- 入口：
  - `https://raw.githubusercontent.com/thenasty1337/free-proxy-list/main/data/latest/proxies.txt`
  - `https://raw.githubusercontent.com/thenasty1337/free-proxy-list/main/data/latest/types/http/proxies.txt`
  - `https://raw.githubusercontent.com/thenasty1337/free-proxy-list/main/data/latest/types/socks4/proxies.txt`
  - `https://raw.githubusercontent.com/thenasty1337/free-proxy-list/main/data/latest/types/socks5/proxies.txt`
- 备注：GitHub 页面发生重定向，接入前需要确认仓库归属和稳定性。

### gfpcom/free-proxy-list

- 类型：GitHub Wiki Raw
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT
- 优先级：中
- 入口：
  - `https://raw.githubusercontent.com/wiki/gfpcom/free-proxy-list/lists/http.txt`
  - `https://raw.githubusercontent.com/wiki/gfpcom/free-proxy-list/lists/https.txt`
  - `https://raw.githubusercontent.com/wiki/gfpcom/free-proxy-list/lists/socks4.txt`
  - `https://raw.githubusercontent.com/wiki/gfpcom/free-proxy-list/lists/socks5.txt`
- 备注：规模很大，需要限量读取、流式解析和严格检测，避免候选池膨胀。

### gfpcom/free-proxy-list sources 目录

- 类型：上游源清单
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5 等
- 格式：TXT，内容是代理源 URL
- 优先级：高，用于“发现代理源”，不直接作为代理列表
- 入口：
  - `https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/http.txt`
  - `https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/https.txt`
  - `https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/socks4.txt`
  - `https://raw.githubusercontent.com/gfpcom/free-proxy-list/main/sources/socks5.txt`
- 代表性新增源：
  - `https://raw.githubusercontent.com/ALIILAPRO/Proxy/main/http.txt`
  - `https://raw.githubusercontent.com/ALIILAPRO/Proxy/main/socks4.txt`
  - `https://raw.githubusercontent.com/ALIILAPRO/Proxy/main/socks5.txt`
  - `https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/http.txt`
  - `https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/socks4.txt`
  - `https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/socks5.txt`
  - `https://raw.githubusercontent.com/SevenworksDev/proxy-list/main/proxies/http.txt`
  - `https://raw.githubusercontent.com/SevenworksDev/proxy-list/main/proxies/socks4.txt`
  - `https://raw.githubusercontent.com/SevenworksDev/proxy-list/main/proxies/socks5.txt`
  - `https://raw.githubusercontent.com/roosterkid/openproxylist/main/HTTPS_RAW.txt`
  - `https://raw.githubusercontent.com/roosterkid/openproxylist/main/SOCKS4_RAW.txt`
  - `https://raw.githubusercontent.com/roosterkid/openproxylist/main/SOCKS5_RAW.txt`
  - `https://raw.githubusercontent.com/Tsprnay/Proxy-lists/master/proxies/http.txt`
  - `https://raw.githubusercontent.com/Tsprnay/Proxy-lists/master/proxies/socks4.txt`
  - `https://raw.githubusercontent.com/Tsprnay/Proxy-lists/master/proxies/socks5.txt`
  - `https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/http.txt`
  - `https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/socks4.txt`
  - `https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/socks5.txt`
  - `https://raw.githubusercontent.com/fyvri/fresh-proxy-list/archive/storage/classic/http.txt`
  - `https://raw.githubusercontent.com/fyvri/fresh-proxy-list/archive/storage/classic/socks5.txt`
- 备注：这些 URL 需要由辅助发现流程去重、抽样访问、打分后再进入候选源配置。

### free-proxy Python 生态提到的页面源

- 类型：HTML 页面
- 协议：HTTP、HTTPS
- 格式：HTML 表格
- 优先级：低
- 入口：
  - `https://www.sslproxies.org/`
  - `https://www.us-proxy.org/`
  - `https://free-proxy-list.net/uk-proxy.html`
  - `https://free-proxy-list.net/`
- 备注：`jundymek/free-proxy` 和同类项目使用这些页面作为代理来源；页面型源容易变动，先不作为 MVP 默认源。

## 辅助代理源发现

很多 GitHub 项目会把代理源写在 README、源码、配置文件、`sources/` 目录或 workflow 里。plugproxy 可以做一个辅助爬虫，但它的职责应该是“发现代理源”，不是直接把发现到的代理投入可用池。

建议能力：

- GitHub 仓库发现：搜索 `free proxy`、`proxy scraper`、`socks5.txt`、`sources/http.txt` 等关键词。
- README/源码扫描：抽取 `raw.githubusercontent.com`、`cdn.jsdelivr.net/gh`、`.txt`、`.json`、`.csv`、API URL。
- 源 URL 分类：按协议、格式、宿主、更新频率、是否需要认证分类。
- 抽样验证：只读取少量字节，判断是否像代理列表或源列表。
- 源评分：根据可访问性、格式稳定性、协议明确性、重复率和最近更新时间打分。
- 人工审核队列：新发现源先进入候选源清单，不自动启用。

建议命令：

```bash
plugproxy discover github --query "free proxy sources" --limit 50
plugproxy discover url https://github.com/gfpcom/free-proxy-list
plugproxy discover validate docs/proxy-sources.candidates.json
```

输出建议：

```json
{
  "name": "example-source",
  "url": "https://raw.githubusercontent.com/example/proxy/main/http.txt",
  "format": "text",
  "protocol_hint": "http",
  "host": "raw.githubusercontent.com",
  "confidence": 0.82,
  "discovered_from": "github:gfpcom/free-proxy-list:sources/http.txt"
}
```

边界：

- 默认只抓 GitHub API、Raw 文件、公开 API 文档和明确导出链接。
- 不绕过登录、验证码、付费墙或 robots 限制。
- 不高频请求公共源。
- 不把未检测代理暴露给用户项目。

## 页面/API 待验证源

### ProxyRadar

- 类型：页面/API
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT、CSV、JSON
- 优先级：中
- 入口：
  - `https://proxyradar.net/`
- 备注：页面说明支持过滤、导出和 API，但需要进一步确认具体接口。

### FreeProxy24

- 类型：页面/API
- 协议：HTTP、HTTPS、SOCKS4、SOCKS5
- 格式：TXT、CSV、JSON
- 优先级：中
- 入口：
  - `https://freeproxy24.com/free-proxy-list`
- 备注：页面说明提供 API 和导出，具体下载接口需要抓包或查看页面脚本。

### litport.net

- 类型：页面/API
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：JSON、CSV、TXT
- 优先级：低
- 入口：
  - `https://litport.net/free-proxy`
- 备注：页面说明 API/导出需要登录或注册，暂不作为默认内置源。

### GimmeProxy

- 类型：公开 API
- 协议：HTTP、SOCKS4、SOCKS5
- 格式：JSON
- 优先级：低
- 入口：
  - `https://gimmeproxy.com/`
- 备注：搜索结果显示 JSON API，但当前访问不稳定，需要后续复核。

### Geonode

- 类型：商业/账号 API
- 协议：HTTP、SOCKS5 等
- 格式：JSON
- 优先级：低
- 入口：
  - `https://docs.geonode.com/api-reference/introduction`
- 备注：需要认证和服务配置，不适合作为默认免费源，但可作为未来付费/账号源适配样例。

## 运行时支持状态

当前 `fetch/list/check/run/doctor` 主链路支持五个通用 adapter：

1. `raw_text_url`
   - 输入：URL、默认协议、source name。
   - 支持格式：
     - `ip:port`
     - `protocol://ip:port`
   - 适配大多数 GitHub Raw、CDN Raw 和 TXT API。

2. `html_text_url` / `br_text_url`
   - 输入：URL、默认协议、source name。
   - 支持格式：
     - 简单 HTML 页面中的 `ip:port`。
     - `<br>` 分隔的接口结果。
     - `<td>IP</td><td>PORT</td>` 这类轻量表格单元格。
     - 显式 `protocol://ip:port`。
   - 适配 89IP、快代理等中文免费代理页面的轻量输出。
   - 不执行 JS，不绕过登录、验证码或付费墙；需要脚本渲染、分页会话或复杂列映射的页面仍后置。

3. `json_url`
   - 输入：URL、默认协议、可选字段映射。
   - 支持 JSON 根结构：
     - 字符串数组，例如 `["1.1.1.1:8080", "http://2.2.2.2:8080"]`。
     - 对象数组，例如 `[{ "ip": "1.1.1.1", "port": 8080, "protocol": "http" }]`。
     - 对象包数组，例如 `{ "proxies": [...] }`、`{ "data": [...] }`、`{ "items": [...] }`、`{ "results": [...] }`。
   - 自动识别字段：
     - 地址字段：`proxy`、`url`、`address`、`addr`。
     - 主机字段：`ip`、`host`。
     - 端口字段：`port`。
     - 协议字段：`protocol`、`type`、`scheme`。
   - 可用 `json.items_path`、`json.proxy_field`、`json.host_field`、`json.port_field`、`json.protocol_field` 覆盖默认映射。
   - `items_path` 只支持单层 key，不实现 JSONPath/JQ。

4. `api_url`
   - 第一版复用 `json_url` 解析器。
   - 额外支持 `headers`，用于设置公开 API 所需的 `Accept`、`User-Agent` 等请求头。
   - 不实现认证流程，不执行 JS，不绕过登录、验证码或付费墙。

复杂页面型 `html_table` 仍后置。页面型源需要独立限速、缓存和失败降级，不能影响主流程。

配置示例：

```json
{
  "name": "89ip-html",
  "type": "html_text_url",
  "url": "https://www.89ip.cn/tqdl.html?api=1&num=100&port=&address=&isp=&anonymity=&type=1",
  "protocol_hint": "http",
  "enabled": false
}
```

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

## 默认内置源建议

MVP 默认只启用这些相对稳定、易解析的源：

- ProxyScrape TXT API
- Proxifly TXT
- dpangestuw TXT
- TheSpeedX TXT
- ProxyScraper TXT
- OpenProxyList TXT

其他源放进示例配置，由用户手动启用。

## 风险

- 免费代理高失效率是常态，不能用采集数量衡量质量。
- 大型源可能重复率高，需要全局去重。
- 同一代理可能被不同源标记为不同协议，需要检测器确认。
- 部分代理可能返回污染内容，需要检测目标做响应指纹校验。
- 公共代理存在隐私和安全风险，默认不应用于敏感流量。

## 参考来源

- Proxifly Free Proxy List: https://github.com/proxifly/free-proxy-list
- dpangestuw/Free-Proxy: https://github.com/dpangestuw/Free-Proxy
- TheSpeedX/PROXY-List: https://github.com/TheSpeedX/PROXY-List
- ProxyScraper: https://proxyscraper.github.io/ProxyScraper/
- ProxyScrape: https://proxyscrape.com/free-proxy-list
- OpenProxyList: https://api.openproxylist.xyz/
- proxy-list.download: https://www.proxy-list.download/api/v1
- monosans/proxy-list: https://github.com/monosans/proxy-list
- joy-deploy/free-proxy-list: https://github.com/joy-deploy/free-proxy-list
- gfpcom/free-proxy-list: https://github.com/gfpcom/free-proxy-list
- ProxyRadar: https://proxyradar.net/
- FreeProxy24: https://freeproxy24.com/free-proxy-list
- litport.net: https://litport.net/free-proxy
- Geonode API Reference: https://docs.geonode.com/api-reference/introduction
