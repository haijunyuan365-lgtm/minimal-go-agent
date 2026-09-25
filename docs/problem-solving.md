# 问题解决记录

## 2026-09-25：SQLite 驱动选择

- 问题：本机 Go 环境没有可用的 `gcc`，依赖 CGO 的 SQLite 驱动会增加构建障碍。
- 处理：使用纯 Go 的 `modernc.org/sqlite`。该依赖不是 Agent 框架，Agent Runtime 仍由项目自行实现。
- 验证：数据库建表、session 持久化及隔离测试通过。

## 2026-09-25：本地 Go 缓存权限

- 问题：受限工作区不能写入默认的 Go module 校验缓存；Go 命令也会打印遥测 token 权限提示。
- 处理：开发验证时把 `GOPATH` 和 `GOCACHE` 指向项目内的 `.go/`，并在 `.gitignore` 中排除该目录。
- 验证：`go mod tidy` 和 `go test ./...` 均退出成功。遥测提示不影响构建和测试。

## 2026-09-25：第一阶段服务检查

- 启动本地服务，`GET /healthz` 返回 `{"status":"ok"}`。
- 调用 `POST /sessions` 创建 session，再用 `GET /sessions/{id}?user_id=user-a` 读取，返回相同的 session ID。
- 验证完成后已停止本地服务；检查产生的数据库文件位于已忽略的 `data/` 目录。
