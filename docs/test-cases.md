# 测试用例与验证记录

## 离线自动测试

运行 `go test ./...`。这些测试使用假 LLM 或本地 HTTP 模拟接口，不需要 API Key，也不会访问模型服务。

| 要求 | 用例 | 验证点 |
| --- | --- | --- |
| 直接回答与纯对话追问 | `TestRunDirectAnswerAndFollowupHistory` | 无工具调用，后续轮次带上同一 session 的历史 |
| 工具注册与 Schema | `TestRegistryAndArgumentValidation` | 注册项包含名称、描述和参数 Schema；拒绝坏参数 |
| 多工具循环与 trace | `TestRunMultipleToolsAndTrace` | 按 `call_id` 回传两个工具结果，记录步骤与结果 |
| 工具失败 | `TestToolErrorsAreTracedAndReturnedToModel` | 除零和无效参数写入错误 trace，模型收到错误输出 |
| 轮次限制 | `TestRunUnknownToolErrorAndLimit` | 未知工具可追踪；达到上限时停止 |
| 工具追问 | `TestToolFollowupReadsSessionTodo` | 下一轮仍可读取当前 session 的待办 |
| 窗口隔离 | `TestTwoWindowsKeepConversationAndTodosSeparate` | 同一用户的两个 session 不共享历史或待办 |
| 长对话与重启 | `TestMemoryCompactionAndRestartFollowup` | 较早轮次压缩为摘要，重启后可召回 |
| DeepSeek 协议 | `TestDeepSeekResponsesWireFormat`、`TestDeepSeekToolLoopAndFollowUp` | 鉴权、工具 Schema、reasoning、结果回传及追问 |
| 配置切换 | `TestLoadFileAndSelectProvider`、`TestConfiguredClientUsesSelectedProvider` | YAML 选择对应模型、端点和密钥引用 |
| API 异常 | `TestResponsesClientTimeout`、`TestResponsesClientRejectsInvalidResponses` | 超时、坏 JSON、缺字段和未完成响应均报错 |
| HTTP 与持久化 | `TestChatAndTraceRoutes`、`TestStorePersistsAndIsolatesSessions` | 聊天、trace 路由及 SQLite 数据隔离 |

2026-09-26 执行 `go test ./...`：所有包通过。真实模型调用尚未执行，原因是 API Key 将由用户最后配置。

## 配置密钥后的验证

1. 在 `config.yml` 中确认 `llm.provider`、对应模型和 `/responses` 端点。
2. 在 `.env` 或系统环境变量中配置对应的 API Key，不要把真实密钥提交到 Git。
3. 运行 `./scripts/live-smoke.ps1`。它会显式启用 `TestLiveResponsesAPI` 和 `TestLiveAgentToolFlow`，分别检查基础模型响应和“计算器 + 模拟天气”工具循环。
4. 运行 `./scripts/run.ps1`，按 README 的示例创建两个 session，并分别验证待办与 trace。

真实 API 验证结果待配置密钥后补充。
