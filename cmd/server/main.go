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

	store, err := session.Open(ctx, cfg.DatabasePath)
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
	var client llm.Client
	model := cfg.OpenAIModel
	if cfg.LLMProvider == "deepseek" {
		model = cfg.DeepSeekModel
		if cfg.DeepSeekAPIKey != "" {
			deepSeek := llm.NewDeepSeekClient(cfg.DeepSeekAPIKey)
			if cfg.DeepSeekEndpoint != "" {
				deepSeek.Endpoint = cfg.DeepSeekEndpoint
			}
			client = deepSeek
		}
	} else if cfg.OpenAIAPIKey != "" {
		client = llm.NewResponsesClient(cfg.OpenAIAPIKey)
	}
	runner := agent.NewRunner(client, store, registry, model)
	if cfg.LLMProvider == "deepseek" {
		runner.ReasoningEffort = cfg.DeepSeekReasoningEffort
	} else {
		runner.ReasoningSummary = cfg.OpenAIReasoningSummary
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpapi.NewHandler(store, runner),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      190 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()
	logger.Info("server starting", "addr", cfg.ListenAddr, "llm_provider", cfg.LLMProvider, "model", model)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
