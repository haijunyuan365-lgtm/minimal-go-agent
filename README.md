# DemoAgent

一个从零实现的 Go Agent 项目。项目骨架、工具调用循环、session 隔离和基础上下文压缩已经实现；支持 OpenAI 和 DeepSeek 的 Responses API。真实 API 连通性将在配置密钥后验证。阶段安排见[实现计划](PLAN.md)。

代码仓库：[haijunyuan365-lgtm/minimal-go-agent](https://github.com/haijunyuan365-lgtm/minimal-go-agent)

## 环境要求

- Go 1.25 或更新版本（当前 SQLite 驱动的要求）
- 运行真实对话时需要所选服务商的 API Key 和支持 function calling 的模型；健康检查和创建 session 不需要 Key

## 启动

在项目根目录编辑 [`config.yml`](config.yml)：`llm.provider` 选 `deepseek` 或 `openai`，对应区块中修改 `model`、`endpoint` 和可选的 reasoning 配置。文件还包含监听地址、数据库路径及 Agent 轮次和上下文限制。当前默认选择 DeepSeek，模型为 `deepseek-flash`。

复制 `.env.example` 为 `.env`，填入所选服务商的 API Key。`config.yml` 的 `api_key: "${DEEPSEEK_API_KEY}"` 会读取该环境变量；OpenAI 同理。`.env` 已被 Git 忽略。也可以直接设置系统环境变量。然后从仓库根目录运行：

```powershell
.\scripts\run.ps1
```

默认监听 `:8080`，数据库位于 `data/agent.db`。如需使用另一份 YAML 文件，设置 `AGENT_CONFIG` 为文件路径。没有密钥时服务仍可启动，但聊天接口返回 HTTP 503。可以在 `config.yml` 的 `api_key` 中直接填写密钥，但该文件受 Git 跟踪，建议保持环境变量引用，避免提交密钥。

DeepSeek 使用官方原生 Responses API，默认地址在 `config.yml` 中为 `https://api.deepseek.com/responses`。`llm.deepseek.model` 可选 `deepseek-flash` 或 `deepseek-v4-pro`；使用兼容代理时修改 `llm.deepseek.endpoint` 为完整的 `/responses` 地址。`reasoning_effort` 可选 `none`、`low`、`high`、`max`，留空时由模型决定。DeepSeek 返回的 reasoning 输出项仅在本轮工具调用继续时回传给模型，不写入跨轮 memory，也不作为摘要返回。DeepSeek 对请求中的 `store`、工具 `strict` 和 reasoning summary 不提供相同语义，适配器会省略这些字段。

要使用 OpenAI，将 `llm.provider` 改成 `openai`，设置 `OPENAI_API_KEY`，并在 `llm.openai` 中选择模型及 `/responses` 地址。支持的模型可把 `reasoning_summary` 设为 `auto`。

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
- `config.yml` 和 `internal/config`：YAML 配置、校验与 API Key 环境变量引用。
- `internal/httpapi`：HTTP 路由和请求校验。
- `internal/llm`：OpenAI/DeepSeek Responses API HTTP 客户端和可替换接口。
- `internal/agent`：输出解析、Agent 主循环、session 锁和上下文压缩。
- `internal/tools`：工具注册、Schema 校验、计算器、模拟搜索与天气、待办。
- `internal/session`：SQLite 建表、session、消息、待办和 trace 存取。

## 设计与 memory

每次收到输入时，程序按 `(user_id, session_id)` 串行处理同一窗口的请求，并读取该窗口的摘要与尚未压缩的完整轮次。模型 context 依次放入固定指令、**该 session 的摘要**、近期完整轮次、本轮用户输入。本轮模型输出项与对应工具结果按原顺序回传，供模型继续决策。待办保存在独立表中，仅在模型调用 `todo` 时读取；工具 trace 和原始 reasoning 不塞入跨轮 context。

当摘要加未压缩消息超过约 12,000 字节，或完整轮次超过 8 轮时，程序先调用 LLM，把较早的已完成轮次合并进最多 2,000 字符的摘要。近期最多保留 8 轮，并用约 8,000 字节的预算调整保留数量；每个用户请求最多执行 3 次压缩。SQLite 记录摘要覆盖到的消息 ID，因此下次只召回边界之后的消息。原始消息继续保留，重启后仍能恢复摘要和近期对话。这里用字节长度做粗略阈值，不是精确 token 计数。压缩失败时不推进摘要边界，也不写入本轮对话。

模型可直接回答，也可调用一个或多个工具。工具必须先注册名称、描述和 JSON Schema；本地执行前还会校验参数。`search` 和 `weather` 返回的是显式标记的模拟数据。单轮最多 6 次 LLM 调用和 8 次工具执行；工具结果或错误写入 SQLite trace。使用 OpenAI 且模型支持 reasoning summary 时，可设置 `OPENAI_REASONING_SUMMARY=auto`；DeepSeek 不生成此类摘要。

## 测试

```powershell
go test ./...
```

配置密钥后，运行真实 API 冒烟测试（会产生 API 调用）：

```powershell
.\scripts\live-smoke.ps1
```

该脚本根据 `config.yml` 中的 `llm.provider` 测试所选真实 Responses API 和“计算器 + 天气”工具循环。没有密钥时，普通 `go test ./...` 使用本地模拟接口和假 LLM，不发起真实 API 调用。普通测试还覆盖配置加载与提供方切换、DeepSeek 请求格式及工具回传、长对话压缩、纯对话追问、待办工具追问、两个窗口隔离、旧数据库升级和重启恢复。

DeepSeek 协议依据：[Responses API 指南](https://api-docs.deepseek.com/guides/responses_api/)及[接口定义](https://api-docs.deepseek.com/api/create-response/)。

开发过程记录在 [AI Prompt 记录](docs/prompts.md)和[问题解决记录](docs/problem-solving.md)。
