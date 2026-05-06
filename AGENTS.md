# AGENTS.md

## 项目说明

plugproxy 是一个 Go 语言轻量代理采集、检测、代理池管理和接入工具。项目目标是先提供稳定 CLI 和 HTTP API，再演进到轻量前端管理面板。

## 文档规则

- 默认使用中文。
- `README.md` 保持仓库入口性质，只放项目简介、快速命令和文档索引。
- 详细文档默认放到 `docs/` 目录。
- 新增设计文档、路线图、规范和方案时，优先创建或更新 `docs/*.md`。

## Go 规则

- 遵循 Google Go 风格。
- 提交前运行 `gofmt`。
- 提交前运行 `go test ./...`。
- 标准库优先，谨慎增加第三方依赖。
- 并发代码必须支持 `context.Context` 取消，避免 goroutine 泄漏。
- 公共模型和 SDK 相关代码放在 `pkg/`，内部实现放在 `internal/`。

## GitHub 规则

- 默认使用 GitHub CLI：`gh`。
- 创建、查看、推送仓库和 PR 时优先使用 `gh`。
- 当前仓库使用 GitHub Actions 作为 CI/CD。
- V1.0 之前以快速推进为主，可以直接提交并推送到 `main`。
- 新版本发布必须创建 annotated tag，例如 `git tag -a v0.1.0 -m "v0.1.0"`。
- V1.0 之后再严格执行：先创建 issue，再基于 issue 创建分支和 PR。
- V1.0 之后 PR 描述需要关联 issue，例如 `Closes #123`。
- V1.0 之后 PR 检查没有问题后启用自动合并，并在合并后删除已合并分支。
- 不要在没有明确要求时改写 Git 历史。
- 不要回滚用户未明确要求回滚的改动。

## 常用命令

```bash
go test ./...
go build -o bin/plugproxy ./cmd/plugproxy
go run ./cmd/plugproxy version
go run ./cmd/plugproxy fetch -cache .plugproxy.cache.json
go run ./cmd/plugproxy list
go run ./cmd/plugproxy get -cache .plugproxy.cache.json -strategy fastest -protocol http -healthy=true
go run ./cmd/plugproxy stats -cache .plugproxy.cache.json
go run ./cmd/plugproxy check -source-workers 32 -workers 128 -protocol http -cache .plugproxy.cache.json
go run ./cmd/plugproxy run -addr 127.0.0.1:8899 -skip-check=false -refresh=true -refresh-interval 5m
go run ./cmd/plugproxy discover search -query "free proxy list socks5" -limit 10
```

## 当前架构

```text
cmd/plugproxy/       CLI 入口
internal/app/        应用编排
internal/cache/      代理缓存
internal/checker/    代理检测
internal/fetcher/    并发代理源采集
internal/pool/       代理池接口与内存实现
internal/server/     轻量 HTTP API
internal/source/     代理源接口与实现
pkg/model/           公开代理数据模型
pkg/client/          HTTP Client SDK
pkg/plugproxy/       嵌入式 SDK
docs/                项目文档
```
