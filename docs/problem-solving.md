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

## 2026-09-25：工具调用协议与可测试性

- 问题：真实模型可返回零个、一个或多个工具调用，也可能在同一响应中带有 reasoning 或中间消息。
- 处理：按 `response.output` 项类型解析；先保存当前响应的全部输出项，再按 `call_id` 追加每个 `function_call_output`。Runtime 依赖 `llm.Client` 接口，单元测试可用脚本化假客户端控制每轮模型行为。
- 验证：多工具、直接回答、追问上下文、未知工具、轮次上限和 HTTP trace 测试通过。
- 手动回放历史回答时保留 `phase: "final_answer"`，避免把已完成回答误当成中间消息。

## 2026-09-25：真实 API 测试待执行

- 当前执行环境未设置 `OPENAI_API_KEY` 和 `OPENAI_MODEL`。用户计划稍后在本机配置密钥。
- 提供 `.env.example` 和 `scripts/live-smoke.ps1`；配置后可运行真实 HTTP 请求与工具循环验证。
- 无密钥的实际服务检查已通过：健康检查和 session 创建正常，trace 可读，聊天返回预期的 HTTP 503。
