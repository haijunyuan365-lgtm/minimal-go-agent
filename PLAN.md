# 最小可用 Agent：Go 实现计划

## 目标与边界

从零实现 Agent Runtime，不使用 Agent 框架。真实 LLM API 负责判断直接回复或调用工具；Go 程序自行管理工具注册、调用循环、会话、上下文、异常和 trace。搜索与天气可以使用明确标记的模拟数据。

## 技术方案

- Go 标准库实现 HTTP API、LLM HTTP 客户端和主循环。
- OpenAI/DeepSeek Responses API 的 function calling：向模型提供工具 JSON Schema，解析 `function_call`，执行本地工具，再提交带相同 `call_id` 的 `function_call_output`。
- SQLite 保存用户、session、消息、摘要、待办和工具 trace；第三方依赖仅用于 SQLite 与 YAML 解析，不使用 Agent 框架。
- `config.yml` 管理模型名称、接口地址、监听地址、数据库路径和运行限制。API Key 使用文件中的环境变量引用，不写入代码或日志。

## 分阶段实施

### 阶段一：项目骨架（已完成）

- 建立 Go module、目录结构和运行说明。
- 配置加载与校验；定义 `LLMClient` 接口，隔离真实 API 和测试替身。
- 建立 HTTP 路由和基本请求/响应结构。
- 建立 SQLite 表和存储接口；启动时执行建表 SQL。
- 用基础测试验证配置、路由和数据库初始化。

**完成标准：** 服务可启动，健康检查可用，能创建 session，能读取该 session 的基础信息；此阶段聊天路由可返回明确的“尚未实现”错误。

已完成配置、SQLite 建表、session 存取、HTTP 路由和自动化测试。真实服务启动检查见开发记录。

### 阶段二：工具和核心循环（代码已实现，待真实 API 验证）

- 定义 `Tool` 接口：名称、描述、参数 JSON Schema、执行函数；实现注册表。
- 实现 `calculator`、模拟 `search`、模拟 `weather`、`todo`。
- 实现 Responses API 客户端和输出解析器，处理 `message`、`function_call`、可选 `reasoning` 项。
- 实现用户输入 → LLM → 工具 → LLM 的循环，支持一次响应中多个工具调用。
- 设置单轮最多 6 次 LLM 调用、8 次工具执行，处理未知工具、坏参数、工具失败、API 超时。

**完成标准：** 使用真实 LLM API 跑通直接回答、一次工具调用和连续工具调用。

代码、假 LLM 测试及可选的真实 API 冒烟测试脚本已就绪。当前执行环境没有 API Key；用户将在本地配置后再验证连通性。

### 阶段三：会话与上下文（已完成，真实 API 验证随阶段二待办）

- 使用 `(user_id, session_id)` 隔离消息、摘要、待办和 trace。服务端生成随机 session ID。
- 每轮开始召回：固定指令 → 当前 session 摘要 → 最近 6～10 个完整轮次 → 当前输入。
- 本轮工具调用与结果完整保留；只压缩已完成的较早轮次。
- 超过上下文阈值时生成简短摘要，原始消息继续保留在数据库中。
- 为同一用户的两个窗口和纯对话、带工具的追问做验证。

**完成标准：** 两个窗口互不串话；重启服务后仍能继续对话和读取待办。

已实现按 session 串行处理、摘要边界持久化与旧库迁移、阈值触发的 LLM 摘要、近期完整轮次回放。假 LLM 测试覆盖两个窗口、纯对话追问、待办工具追问、压缩和重启恢复。真实 API 连通性按用户安排留到本机配置密钥后测试。

### 阶段四：测试、文档与提交

- 假 `LLMClient` 测循环、错误、轮次上限和 session 隔离；真实 API 冒烟测试由 `config.yml` 所选提供方的 API Key 显式启用。
- 增加脱敏工具 trace，记录时间、session、轮次、工具名、参数、结果/错误、耗时。
- README 写清运行方式、系统设计、memory 召回时机与放置方式、测试和演示命令。
- `docs/prompts.md` 记录 AI Prompt；`docs/problem-solving.md` 记录遇到的问题与解决方式。
- 提交 GitHub，并在 README 中提供仓库链接与真实 API 演示记录。

## 预期接口

- `GET /healthz`：健康检查。
- `POST /sessions`：传入 `user_id`，创建 session。
- `GET /sessions/{id}?user_id=...`：读取 session 基础信息。
- `POST /sessions/{id}/messages`：发送消息并运行 Agent Loop。
- `GET /sessions/{id}/trace?user_id=...`：查看该 session 工具执行记录。

## 核心测试用例

| 用例 | 预期 |
| --- | --- |
| “你好” | 直接回复，无工具调用 |
| “计算 (12+8)/4” | 调用 calculator，返回 5 |
| “搜索项目说明并总结” | 调用 search，引用模拟结果 |
| “查北京天气并记一个带伞的待办” | 完成多次工具调用 |
| 同一 session 追问待办 | 读取之前的状态 |
| 同一用户两个 session 各记待办 | 数据互不影响 |
| 长对话后追问早期事实 | 摘要支持回答 |
| 坏参数、除零、API 超时 | 返回可理解的错误，有 trace |
| 模型持续请求工具 | 达到轮次上限后停止 |
| 服务重启后继续 session | 数据仍在 |

## 设计注意点

- “思考过程”只记录 API 实际返回的 reasoning summary（若可用）、工具决策和执行轨迹，不假设可以获取模型内部完整推理。
- 搜索和天气的模拟结果必须显式标注，不能作为实时事实呈现。
- 工具参数在执行前按 Schema 校验；计算器只解析允许的算术表达式，不执行任意代码。
- 仓库中的 `config.yml` 使用环境变量引用获取 API Key；trace 不能包含密钥和完整请求头。

## 官方资料

- [OpenAI Function calling](https://developers.openai.com/api/docs/guides/function-calling)
- [OpenAI Conversation state](https://developers.openai.com/api/docs/guides/conversation-state)
- [OpenAI Reasoning](https://developers.openai.com/api/docs/guides/reasoning)
- [DeepSeek Responses API](https://api-docs.deepseek.com/guides/responses_api/)
