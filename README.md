# DemoAgent

一个从零实现的 Go Agent 项目。当前已完成[计划](PLAN.md)中的**阶段一：项目骨架**；工具调用与 Agent Loop 将在阶段二实现。

代码仓库：[haijunyuan365-lgtm/minimal-go-agent](https://github.com/haijunyuan365-lgtm/minimal-go-agent)

## 环境要求

- Go 1.25 或更新版本（当前 SQLite 驱动的要求）
- 运行真实对话时需要 OpenAI API Key；阶段一的健康检查和 session 接口不需要 Key

## 启动

```powershell
cd minimal-go-agent
$env:AGENT_LISTEN_ADDR = ':8080'
$env:AGENT_DB_PATH = 'data/agent.db'
go run ./cmd/server
```

创建并读取 session：

```powershell
Invoke-RestMethod http://localhost:8080/healthz
$session = Invoke-RestMethod -Method Post -Uri http://localhost:8080/sessions -ContentType 'application/json' -Body '{"user_id":"user-a"}'
Invoke-RestMethod "http://localhost:8080/sessions/$($session.session_id)?user_id=user-a"
```

发送消息接口在阶段一返回 HTTP 501；完成阶段二后再设置 `OPENAI_API_KEY` 和 `OPENAI_MODEL` 运行真实对话。

## 当前结构

- `cmd/server`：HTTP 服务入口。
- `internal/config`：环境变量配置。
- `internal/httpapi`：HTTP 路由和请求校验。
- `internal/llm`：LLM 客户端接口，供后续真实适配器和测试替身实现。
- `internal/session`：SQLite 建表及 session 存取。

## 设计与 memory

详细设计和阶段验收标准见 [PLAN.md](PLAN.md)。在阶段三，每次收到输入时，将当前 session 的摘要和近期完整轮次放在本轮用户输入之前；待办保存在结构化表中，工具调用与结果在本轮保持成对。只有已完成的较早轮次会进入摘要。不同 session 通过 `(user_id, session_id)` 隔离。

## 测试

```powershell
go test ./...
```

开发过程持续记录在 [AI Prompt 记录](docs/prompts.md)和[问题解决记录](docs/problem-solving.md)。真实 API 演示将在阶段二完成。
