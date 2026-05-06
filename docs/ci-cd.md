# GitHub Actions CI/CD

本项目使用 GitHub Actions 作为默认 CI/CD。

## 目标

- PR 自动运行 Go 检查。
- tag 推送自动构建并发布跨平台二进制。
- 检查通过后可以启用自动合并。
- 合并后自动删除功能分支。
- 所有 GitHub 操作默认使用 GitHub CLI，也就是 `gh`。

## 推荐流程

```text
issue -> branch -> PR -> GitHub Actions 检查 -> 自动合并 -> 删除分支
```

## 当前检查项

普通 push 和 PR 会运行 CI workflow：

- `gofmt` 检查。
- `go test ./...`。
- `go build ./cmd/plugproxy`。

`v*` tag 推送会运行 Release workflow：

- `go test ./...`。
- 使用 `ldflags` 注入版本、commit 和构建时间。
- 构建 Windows、Linux、macOS 的 amd64/arm64 二进制。
- 上传压缩包和 `checksums.txt` 到 GitHub Release。

## 常用命令

```bash
gh auth status
gh issue create
gh pr create
gh pr checks
gh pr merge --auto --squash --delete-branch
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
gh release view v0.1.0 --web
```

## 约定

- 非临时改动默认先创建 issue。
- PR 描述需要关联 issue，例如 `Closes #123`。
- CI 通过后再自动合并。
- 不在本地绕过 CI 直接合并重要改动。
- 新版本发布必须使用 annotated tag，例如 `v0.1.0`。
- tag 推送前先确认 `main` 已经包含对应发布提交。
