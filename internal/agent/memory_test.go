package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

func TestMemoryCompactionAndRestartFollowup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "agent.db")
	store, err := session.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := store.Create(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range [][2]string{
		{"我的名字叫小海", "记住了你的名字。"},
		{"我喜欢 Go", "记住了你的偏好。"},
		{"今天准备写周报", "好的。"},
		{"本周做了 Agent 项目", "了解。"},
	} {
		if err := store.AddTurn(ctx, "user-a", conversation.ID, turn[0], turn[1]); err != nil {
			t.Fatal(err)
		}
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Calculator{}); err != nil {
		t.Fatal(err)
	}
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{finalMessage("用户叫小海，喜欢 Go。")}},
		{Output: []json.RawMessage{finalMessage("你叫小海。")}},
	}}
	runner := NewRunner(client, store, registry, "test-model")
	runner.MaxRecentTurns = 2
	result, err := runner.Run(ctx, "user-a", conversation.ID, "我叫什么？")
	if err != nil || result.MemoryCompactions != 1 || result.Answer != "你叫小海。" {
		t.Fatalf("compaction result: %+v, %v", result, err)
	}
	if len(client.requests) != 2 || len(client.requests[0].Tools) != 0 || len(client.requests[1].Input) != 6 {
		t.Fatalf("unexpected memory calls: %+v", client.requests)
	}
	if !strings.Contains(string(client.requests[0].Input[0]), "我的名字叫小海") ||
		strings.Contains(string(client.requests[0].Input[0]), "本周做了 Agent 项目") {
		t.Fatalf("wrong transcript compacted: %s", client.requests[0].Input[0])
	}
	if !strings.Contains(string(client.requests[1].Input[0]), "用户叫小海") ||
		strings.Contains(string(client.requests[1].Input[1]), "我的名字叫小海") {
		t.Fatalf("summary was not placed before recent turns: %+v", client.requests[1].Input)
	}
	updated, err := store.Get(ctx, "user-a", conversation.ID)
	if err != nil || updated.SummaryThroughMessageID == 0 || updated.Summary == "" {
		t.Fatalf("summary not persisted: %+v, %v", updated, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := session.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	nextClient := &scriptedClient{responses: []llm.Response{{Output: []json.RawMessage{finalMessage("你叫小海。")}}}}
	nextRunner := NewRunner(nextClient, reopened, registry, "test-model")
	if _, err := nextRunner.Run(ctx, "user-a", conversation.ID, "重启后你还记得我的名字吗？"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nextClient.requests[0].Input[0]), "用户叫小海") {
		t.Fatalf("restarted session lost memory: %+v", nextClient.requests[0].Input)
	}
}

func TestTwoWindowsKeepConversationAndTodosSeparate(t *testing.T) {
	ctx := context.Background()
	store, err := session.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, _ := store.Create(ctx, "user-a")
	second, _ := store.Create(ctx, "user-a")
	if err := store.AddTurn(ctx, "user-a", first.ID, "窗口一：北京天气", "这是演示天气。"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTurn(ctx, "user-a", second.ID, "窗口二：写周报", "好的。"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddTodo(ctx, "user-a", first.ID, "带伞"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddTodo(ctx, "user-a", second.ID, "提交周报"); err != nil {
		t.Fatal(err)
	}
	client := &scriptedClient{responses: []llm.Response{
		{Output: []json.RawMessage{finalMessage("窗口一继续")}},
		{Output: []json.RawMessage{finalMessage("窗口二继续")}},
	}}
	registry := tools.NewRegistry()
	runner := NewRunner(client, store, registry, "test-model")
	if _, err := runner.Run(ctx, "user-a", first.ID, "继续窗口一"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(ctx, "user-a", second.ID, "继续窗口二"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(client.requests[0].Input[0]), "北京天气") ||
		strings.Contains(string(client.requests[0].Input[0]), "写周报") ||
		!strings.Contains(string(client.requests[1].Input[0]), "写周报") ||
		strings.Contains(string(client.requests[1].Input[0]), "北京天气") {
		t.Fatalf("window context leaked: %+v", client.requests)
	}
	firstTodos, _ := store.ListTodos(ctx, "user-a", first.ID)
	secondTodos, _ := store.ListTodos(ctx, "user-a", second.ID)
	if len(firstTodos) != 1 || firstTodos[0].Text != "带伞" || len(secondTodos) != 1 || secondTodos[0].Text != "提交周报" {
		t.Fatalf("window todos leaked: %+v, %+v", firstTodos, secondTodos)
	}
}

func TestFailedCompactionLeavesRawHistoryIntact(t *testing.T) {
	ctx := context.Background()
	client := &scriptedClient{}
	runner, store, conversation := testRunner(t, client)
	runner.MaxRecentTurns = 1
	for _, text := range []string{"fact one", "fact two", "fact three"} {
		if err := store.AddTurn(ctx, "user-a", conversation.ID, text, "noted"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runner.Run(ctx, "user-a", conversation.ID, "follow up"); err == nil {
		t.Fatal("expected summarizer failure")
	}
	current, err := store.Get(ctx, "user-a", conversation.ID)
	if err != nil || current.SummaryThroughMessageID != 0 {
		t.Fatalf("memory advanced after failure: %+v, %v", current, err)
	}
	all, err := store.MessagesAfter(ctx, "user-a", conversation.ID, 0)
	if err != nil || len(all) != 6 {
		t.Fatalf("failed turn was persisted or history lost: %+v, %v", all, err)
	}
}

func TestCharacterBudgetCompactsOnlyCompletedOlderTurns(t *testing.T) {
	messages := []session.Message{
		{ID: 1, Role: "user", Content: strings.Repeat("a", 30)},
		{ID: 2, Role: "assistant", Content: strings.Repeat("b", 30)},
		{ID: 3, Role: "user", Content: strings.Repeat("c", 30)},
		{ID: 4, Role: "assistant", Content: strings.Repeat("d", 30)},
		{ID: 5, Role: "user", Content: strings.Repeat("e", 30)},
		{ID: 6, Role: "assistant", Content: strings.Repeat("f", 30)},
	}
	if !shouldCompact("", messages, 8, 100) {
		t.Fatal("character threshold did not trigger compaction")
	}
	cut := compactionCut(messages, 8, 65)
	if cut != 4 || messages[cut-1].Role != "assistant" {
		t.Fatalf("cut = %d, want completed prefix ending at 4", cut)
	}
}
