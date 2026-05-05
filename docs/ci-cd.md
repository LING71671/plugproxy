# GitHub Actions CI/CD

本项目使用 GitHub Actions 作为默认 CI/CD。

## 目标

- PR 自动运行 Go 检查。
- 检查通过后可以启用自动合并。
- 合并后自动删除功能分支。
- 所有 GitHub 操作默认使用 GitHub CLI，也就是 `gh`。

## 推荐流程

```text
issue -> branch -> PR -> GitHub Actions 检查 -> 自动合并 -> 删除分支
```

## 当前检查项

- `gofmt` 检查。
- `go test ./...`。
- `go build ./cmd/plugproxy`。

## 常用命令

```bash
gh auth status
gh issue create
gh pr create
gh pr checks
gh pr merge --auto --squash --delete-branch
```

## 约定

- 非临时改动默认先创建 issue。
- PR 描述需要关联 issue，例如 `Closes #123`。
- CI 通过后再自动合并。
- 不在本地绕过 CI 直接合并重要改动。
