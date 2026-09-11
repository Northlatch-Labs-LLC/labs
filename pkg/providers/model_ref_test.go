package providers

import "testing"

func TestParseModelRef_WithoutSlash(t *testing.T) {
	ref := ParseModelRef("gpt-4", "openai")
	if ref == nil {
		t.Fatal("expected non-nil ref")
	}
	if ref.Provider != "openai" {
		t.Errorf("provider = %q, want openai", ref.Provider)
	}
	if ref.Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", ref.Model)
	}
}

func TestParseModelRef_Empty(t *testing.T) {
	ref := ParseModelRef("", "openai")
	if ref != nil {
		t.Errorf("expected nil for empty string, got %+v", ref)
	}
}

func TestParseModelRef_EmptyModelAfterSlash(t *testing.T) {
	ref := ParseModelRef("openai/", "default")
	if ref != nil {
		t.Errorf("expected nil for empty model, got %+v", ref)
	}
}

func TestParseModelRef_UnknownPrefixFallsBackToDefaultProvider(t *testing.T) {
	ref := ParseModelRef("meta-llama/Llama-3.1-8B-Instruct", "openai")
	if ref == nil {
		t.Fatal("expected non-nil ref")
	}
	if ref.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", ref.Provider)
	}
	if ref.Model != "meta-llama/Llama-3.1-8B-Instruct" {
		t.Fatalf("model = %q, want full original model ID", ref.Model)
	}
}

func TestParseModelRef_UnknownPrefixPreservesEmptyDefaultProvider(t *testing.T) {
	ref := ParseModelRef("meta-llama/Llama-3.1-8B-Instruct", "")
	if ref == nil {
		t.Fatal("expected non-nil ref")
	}
	if ref.Provider != "" {
		t.Fatalf("provider = %q, want empty", ref.Provider)
	}
	if ref.Model != "meta-llama/Llama-3.1-8B-Instruct" {
		t.Fatalf("model = %q, want full original model ID", ref.Model)
	}
}
