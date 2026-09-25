# DemoAgent

一个从零实现的 Go Agent 项目。已实现项目骨架和第二阶段的工具调用代码；真实 API 连通性将在配置密钥后验证。后续的上下文摘要压缩见[实现计划](PLAN.md)。

代码仓库：[haijunyuan365-lgtm/minimal-go-agent](https://github.com/haijunyuan365-lgtm/minimal-go-agent)

## 环境要求

- Go 1.25 或更新版本（当前 SQLite 驱动的要求）
- 运行真实对话时需要可用的 OpenAI API Key 和支持 function calling 的模型；健康检查和创建 session 不需要 Key

## 启动

复制 `.env.example` 为 `.env`，填入自己的 `OPENAI_API_KEY` 和 `OPENAI_MODEL`。`.env` 已被 Git 忽略。也可以直接设置同名环境变量。然后从仓库根目录运行：

```powershell
.\scripts\run.ps1
```

默认监听 `:8080`，数据库位于 `data/agent.db`。可用 `AGENT_LISTEN_ADDR`、`AGENT_DB_PATH` 覆盖。没有密钥时服务仍可启动，但聊天接口返回 HTTP 503。

创建 session、聊天和查看 trace：

```powershell
Invoke-RestMethod http://localhost:8080/healthz
$session = Invoke-RestMethod -Method Post -Uri http://localhost:8080/sessions -ContentType 'application/json' -Body '{"user_id":"user-a"}'
Invoke-RestMethod "http://localhost:8080/sessions/$($session.session_id)?user_id=user-a"
$body = @{ user_id = 'user-a'; message = '计算 (12+8)/4，再查北京的演示天气' } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/sessions/$($session.session_id)/messages" -ContentType 'application/json' -Body $body
Invoke-RestMethod "http://localhost:8080/sessions/$($session.session_id)/trace?user_id=user-a"
```

另开一个窗口时，再调用一次 `POST /sessions` 获取新 ID。相同用户的两个 session 分别保存消息、待办和 trace。`user_id` 当前只是演示用标识符，不是身份认证。

## 当前结构

- `cmd/server`：HTTP 服务入口。
- `internal/config`：环境变量配置。
- `internal/httpapi`：HTTP 路由和请求校验。
- `internal/llm`：真实 Responses API HTTP 客户端和可替换接口。
- `internal/agent`：输出解析和 Agent 主循环。
- `internal/tools`：工具注册、Schema 校验、计算器、模拟搜索与天气、待办。
- `internal/session`：SQLite 建表、session、消息、待办和 trace 存取。

## 设计与 memory

每次收到输入时，当前实现从 `(user_id, session_id)` 读取最近 12 条用户和 Agent 消息，将它们放在本轮用户输入之前。当前轮的原始模型输出项和对应工具结果会按顺序回传给 Responses API；待办保存在独立的结构化表中，不依赖模型记忆。阶段三将加入摘要压缩：每轮开始召回当前 session 的摘要，放在近期消息之前，只压缩已完成的较早轮次。详细验收标准见 [PLAN.md](PLAN.md)。

模型可直接回答，也可调用一个或多个工具。工具必须先注册名称、描述和 JSON Schema；本地执行前还会校验参数。`search` 和 `weather` 返回的是显式标记的模拟数据。单轮最多 6 次 LLM 调用和 8 次工具执行；工具结果或错误写入 SQLite trace。若所用模型支持 reasoning summary，可在环境中设置 `OPENAI_REASONING_SUMMARY=auto`，响应会返回 API 实际提供的摘要；原始内部推理不可获取。

## 测试

```powershell
go test ./...
```

配置密钥后，运行真实 API 冒烟测试（会产生 API 调用）：

```powershell
.\scripts\live-smoke.ps1
```

该脚本测试真实 HTTP 接口和“计算器 + 天气”工具循环。没有密钥时，普通 `go test ./...` 使用假 LLM，不发起真实 API 调用。

开发过程记录在 [AI Prompt 记录](docs/prompts.md)和[问题解决记录](docs/problem-solving.md)。
