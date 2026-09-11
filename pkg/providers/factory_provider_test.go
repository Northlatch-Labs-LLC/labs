// Labs - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Labs contributors

package providers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Northlatch-Labs-LLC/labs/pkg/config"
)

func TestCreateProviderFromConfig_OpenAI(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-openai",
		Model:     "openai/gpt-4o",
		APIBase:   "https://api.example.com/v1",
	}
	cfg.SetAPIKey("test-key")

	provider, modelID, err := CreateProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateProviderFromConfig() error = %v", err)
	}
	if provider == nil {
		t.Fatal("CreateProviderFromConfig() returned nil provider")
	}
	if modelID != "gpt-4o" {
		t.Errorf("modelID = %q, want %q", modelID, "gpt-4o")
	}
}

func TestCreateProviderFromConfig_PreservesExplicitProviderPrefixedModel(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-openai",
		Provider:  "openai",
		Model:     "openai/gpt-4o",
		APIBase:   "https://api.example.com/v1",
	}
	cfg.SetAPIKey("test-key")

	provider, modelID, err := CreateProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateProviderFromConfig() error = %v", err)
	}
	if provider == nil {
		t.Fatal("CreateProviderFromConfig() returned nil provider")
	}
	if modelID != "openai/gpt-4o" {
		t.Fatalf("modelID = %q, want %q", modelID, "openai/gpt-4o")
	}
}

func TestCreateProviderFromConfig_DefaultAPIBase(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
	}{
		{"openai", "openai"},
		{"venice", "venice"},
		{"nearai", "nearai"},
		{"groq", "groq"},
		{"novita", "novita"},
		{"openrouter", "openrouter"},
		{"cerebras", "cerebras"},
		{"vivgrid", "vivgrid"},
		{"siliconflow", "siliconflow"},
		{"qwen", "qwen"},
		{"vllm", "vllm"},
		{"deepseek", "deepseek"},
		{"ollama", "ollama"},
		{"lmstudio", "lmstudio"},
		{"gpt4free", "gpt4free"},
		{"longcat", "longcat"},
		{"modelscope", "modelscope"},
		{"mimo", "mimo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ModelConfig{
				ModelName: "test-" + tt.protocol,
				Model:     tt.protocol + "/test-model",
			}
			cfg.SetAPIKey("test-key")

			provider, _, err := CreateProviderFromConfig(cfg)
			if err != nil {
				t.Fatalf("CreateProviderFromConfig() error = %v", err)
			}

			// Verify we got an HTTPProvider for all these protocols
			if _, ok := provider.(*HTTPProvider); !ok {
				t.Fatalf("expected *HTTPProvider, got %T", provider)
			}
		})
	}
}

func TestCreateProviderFromConfig_GeminiMissingAPIKey(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-gemini-no-key",
		Model:     "gemini/gemini-2.5-flash",
	}

	_, _, err := CreateProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("CreateProviderFromConfig() expected error for missing gemini API key")
	}
}

func TestCreateProviderFromConfig_MissingAPIKey(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-no-key",
		Model:     "openai/gpt-4o",
	}

	_, _, err := CreateProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("CreateProviderFromConfig() expected error for missing API key")
	}
}

func TestCreateProviderFromConfig_UnknownProtocol(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-unknown-provider",
		Provider:  "unknown-protocol",
		Model:     "model",
	}
	cfg.SetAPIKey("test-key")

	_, _, err := CreateProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("CreateProviderFromConfig() expected error for unknown protocol")
	}
}

func TestCreateProviderFromConfig_UnknownModelPrefixDefaultsToOpenAI(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-unknown-model-prefix",
		Model:     "meta-llama/Llama-3.1-8B-Instruct",
		APIBase:   "https://api.example.com/v1",
	}
	cfg.SetAPIKey("test-key")

	provider, modelID, err := CreateProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateProviderFromConfig() error = %v", err)
	}
	if provider == nil {
		t.Fatal("CreateProviderFromConfig() returned nil provider")
	}
	if modelID != "meta-llama/Llama-3.1-8B-Instruct" {
		t.Fatalf("modelID = %q, want full model ID", modelID)
	}
}

func TestCreateProviderFromConfig_NilConfig(t *testing.T) {
	_, _, err := CreateProviderFromConfig(nil)
	if err == nil {
		t.Fatal("CreateProviderFromConfig(nil) expected error")
	}
}

func TestCreateProviderFromConfig_EmptyModel(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "test-empty",
		Model:     "",
	}

	_, _, err := CreateProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("CreateProviderFromConfig() expected error for empty model")
	}
}

func TestCreateProviderFromConfig_RequestTimeoutPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	cfg := &config.ModelConfig{
		ModelName:      "test-timeout",
		Model:          "openai/gpt-4o",
		APIBase:        server.URL,
		RequestTimeout: 1,
	}
	cfg.SetAPIKey("test-key")

	provider, modelID, err := CreateProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateProviderFromConfig() error = %v", err)
	}
	if modelID != "gpt-4o" {
		t.Fatalf("modelID = %q, want %q", modelID, "gpt-4o")
	}

	_, err = provider.Chat(
		t.Context(),
		[]Message{{Role: "user", Content: "hi"}},
		nil,
		modelID,
		nil,
	)
	if err == nil {
		t.Fatal("Chat() expected timeout error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "context deadline exceeded") && !strings.Contains(errMsg, "Client.Timeout exceeded") {
		t.Fatalf("Chat() error = %q, want timeout-related error", errMsg)
	}
}

func TestCreateProviderFromConfig_AzureMissingAPIKey(t *testing.T) {
	cfg := &config.ModelConfig{
		ModelName: "azure-gpt5",
		Model:     "azure/my-gpt5-deployment",
		APIBase:   "https://my-resource.openai.azure.com",
	}

	_, _, err := CreateProviderFromConfig(cfg)
	// Without api_key the factory falls back to identity auth, which in the
	// default build is stubbed out and surfaces a build-tag error. With the
	// azidentity tag, the call succeeds and is covered by a separate test.
	if err != nil && !strings.Contains(err.Error(), "azidentity") {
		t.Fatalf("CreateProviderFromConfig() unexpected error = %v", err)
	}
}

func TestBuildModelProviderAliasMap(t *testing.T) {
	aliases := buildModelProviderAliasMap()
	if len(aliases) == 0 {
		t.Fatal("buildModelProviderAliasMap() returned empty map")
	}

	seenAliases := make(map[string]string, len(aliases))
	for provider, option := range modelProviderOptionsByName {
		got, ok := aliases[provider]
		if !ok {
			t.Fatalf("canonical provider %q missing from alias map", provider)
		}
		if got != provider {
			t.Fatalf("canonical provider %q mapped to %q", provider, got)
		}
		if existing, ok := seenAliases[provider]; ok {
			t.Fatalf("canonical provider key %q collides with provider %q", provider, existing)
		}
		seenAliases[provider] = provider
		for _, alias := range option.Aliases {
			normalized := strings.ToLower(strings.TrimSpace(alias))
			if normalized == "" {
				t.Fatalf("provider %q includes empty alias", provider)
			}
			if existing, ok := seenAliases[normalized]; ok && existing != provider {
				t.Fatalf("alias %q for provider %q collides with provider %q", alias, provider, existing)
			}
			seenAliases[normalized] = provider
			got, ok := aliases[normalized]
			if !ok {
				t.Fatalf("alias %q for provider %q missing from alias map", alias, provider)
			}
			if got != provider {
				t.Fatalf("alias %q normalized to %q, want %q", alias, got, provider)
			}
		}
	}
}

func TestCreateProviderFromConfig_CustomHeaders(t *testing.T) {
	var gotSource, gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("X-Source")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	cfg := &config.ModelConfig{
		ModelName:     "test-headers",
		Model:         "openai/gpt-4o",
		APIBase:       server.URL,
		CustomHeaders: map[string]string{"X-Source": "coding-plan", "Authorization": "Token config-auth"},
	}
	cfg.SetAPIKey("test-key")

	provider, modelID, err := CreateProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateProviderFromConfig() error = %v", err)
	}

	_, err = provider.Chat(
		t.Context(),
		[]Message{{Role: "user", Content: "hi"}},
		nil,
		modelID,
		nil,
	)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if gotSource != "coding-plan" {
		t.Fatalf("X-Source = %q, want %q", gotSource, "coding-plan")
	}
	if gotAuth != "Token config-auth" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Token config-auth")
	}
}

// openaiCompatResponse is the JSON response used by OpenAI-compatible providers.
const openaiCompatResponse = `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`

// anthropicResponse is the JSON response used by Anthropic providers.
const anthropicResponse = `{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":5}}`

func TestCreateProviderFromConfig_UserAgent(t *testing.T) {
	defaultUA := "Labs/" + config.Version

	tests := []struct {
		name      string
		model     string
		userAgent string
		apiKey    string
		response  string
		wantUA    string
		chatOpts  map[string]any
	}{
		{
			name:     "openai default user agent",
			model:    "openai/gpt-4o",
			apiKey:   "test-key",
			response: openaiCompatResponse,
			wantUA:   defaultUA,
		},
		{
			name:      "openai custom user agent",
			model:     "openai/gpt-4o",
			apiKey:    "test-key",
			userAgent: "MyAgent/1.2.3",
			response:  openaiCompatResponse,
			wantUA:    "MyAgent/1.2.3",
		},
		{
			name:     "anthropic default user agent",
			model:    "anthropic/claude-sonnet-4-20250514",
			apiKey:   "test-key",
			response: anthropicResponse,
			wantUA:   defaultUA,
		},
		{
			name:     "anthropic-messages default user agent",
			model:    "anthropic-messages/claude-sonnet-4-20250514",
			apiKey:   "test-key",
			response: anthropicResponse,
			wantUA:   defaultUA,
			chatOpts: map[string]any{"max_tokens": 1024},
		},
		{
			name:     "azure default user agent",
			model:    "azure/my-deployment",
			apiKey:   "test-azure-key",
			response: openaiCompatResponse,
			wantUA:   defaultUA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedUA string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedUA = r.Header.Get("User-Agent")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			cfg := &config.ModelConfig{
				ModelName: "test-ua-" + tt.name,
				Model:     tt.model,
				APIBase:   server.URL,
				UserAgent: tt.userAgent,
			}
			cfg.SetAPIKey(tt.apiKey)

			provider, modelID, err := CreateProviderFromConfig(cfg)
			if err != nil {
				t.Fatalf("CreateProviderFromConfig() error = %v", err)
			}
			if provider == nil {
				t.Fatal("CreateProviderFromConfig() returned nil provider")
			}

			_, err = provider.Chat(
				t.Context(),
				[]Message{{Role: "user", Content: "hi"}},
				nil,
				modelID,
				tt.chatOpts,
			)
			if err != nil {
				t.Fatalf("Chat() error = %v", err)
			}

			if receivedUA != tt.wantUA {
				t.Errorf("User-Agent = %q, want %q", receivedUA, tt.wantUA)
			}
		})
	}
}
