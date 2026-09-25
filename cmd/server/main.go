package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"demoagent/internal/agent"
	"demoagent/internal/config"
	"demoagent/internal/httpapi"
	"demoagent/internal/llm"
	"demoagent/internal/session"
	"demoagent/internal/tools"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := session.Open(ctx, cfg.Server.DatabasePath)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	registry := tools.NewRegistry()
	for _, tool := range []tools.Tool{tools.Calculator{}, tools.Search{}, tools.Weather{}, tools.TodoTool{Store: store}} {
		if err := registry.Register(tool); err != nil {
			logger.Error("tool registration failed", "error", err)
			os.Exit(1)
		}
	}
	selected := cfg.SelectedLLM()
	runner := agent.NewRunner(llm.NewConfiguredClient(cfg), store, registry, selected.Model, agent.Limits{
		MaxLLMCalls: cfg.Agent.MaxLLMCalls, MaxToolCalls: cfg.Agent.MaxToolCalls,
		MaxMessageChars: cfg.Agent.MaxMessageChars, MaxRecentTurns: cfg.Agent.MaxRecentTurns,
		ContextCharLimit: cfg.Agent.ContextCharLimit, RecentCharBudget: cfg.Agent.RecentCharBudget,
		MaxSummaryChars: cfg.Agent.MaxSummaryChars, MaxCompactionCalls: cfg.Agent.MaxCompactionCalls,
	})
	if cfg.LLM.Provider == "deepseek" {
		runner.ReasoningEffort = selected.ReasoningEffort
	} else {
		runner.ReasoningSummary = selected.ReasoningSummary
	}

	server := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           httpapi.NewHandler(store, runner),
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()
	logger.Info("server starting", "addr", cfg.Server.ListenAddr, "llm_provider", cfg.LLM.Provider, "model", selected.Model)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
