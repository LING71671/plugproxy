# 发布流程

plugproxy 在 v1.0 前允许直接推进 `main`，但正式版本发布必须打 tag。当前发布目标是 `v0.2.1`。

## 发布前检查

```bash
go test ./...
go build ./cmd/plugproxy
```

本地验证版本注入：

```powershell
$commit = git rev-parse --short HEAD
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
go build -ldflags "-X main.version=v0.2.1 -X main.commit=$commit -X main.date=$date" -o bin\plugproxy.exe ./cmd/plugproxy
bin\plugproxy.exe version
```

期望输出类似：

```text
v0.2.1 commit=abcdef0 date=2026-05-06T00:00:00Z
```

## 创建发布

先确认 `main` 已推送并且工作区干净：

```bash
git status --short --branch
git push origin main
```

创建 annotated tag 并推送：

```bash
git tag -a v0.2.1 -m "v0.2.1"
git push origin v0.2.1
```

推送 tag 后，GitHub Actions 会运行 Release workflow，构建并发布以下产物：

- `plugproxy_v0.2.1_windows_amd64.zip`
- `plugproxy_v0.2.1_windows_arm64.zip`
- `plugproxy_v0.2.1_linux_amd64.tar.gz`
- `plugproxy_v0.2.1_linux_arm64.tar.gz`
- `plugproxy_v0.2.1_darwin_amd64.tar.gz`
- `plugproxy_v0.2.1_darwin_arm64.tar.gz`
- `checksums.txt`

查看 release：

```bash
gh release view v0.2.1 --web
gh run list --workflow Release
```

## 错误 tag 处理

如果 tag 推错且 Release 尚未被用户使用，可以删除远端 tag 和 release 后重发：

```bash
gh release delete v0.2.1 --cleanup-tag
git tag -d v0.2.1
```

如果 release 已经被外部使用，不要重写同名 tag，应发布下一个补丁版本，例如 `v0.2.2`。
