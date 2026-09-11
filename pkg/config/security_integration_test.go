// Labs - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Labs contributors

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test JSON unmarshal of private fields (unexported fields are never filled, with or without json tag).
func TestJSONUnmarshalPrivateFields(t *testing.T) {
	type testStruct struct {
		PublicField  string `json:"public"`
		privateField string
	}

	data := `{"public": "pub", "privateField": "priv"}`
	var s testStruct
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	t.Logf("PublicField: %s", s.PublicField)
	t.Logf("privateField: %s", s.privateField)

	if s.PublicField != "pub" {
		t.Errorf("PublicField = %q, want 'pub'", s.PublicField)
	}
	if s.privateField != "" {
		t.Errorf("privateField = %q, want empty because unexported fields are ignored", s.privateField)
	}
}

func TestSecurityConfigWithAPIKeysArray(t *testing.T) {
	t.Run("Multiple API keys via security", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create config with APIKeys array
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
  "version": 1,
  "model_list": [
    {
      "model_name": "multi-key-model",
      "model": "openai/multi-key-model"
    }
  ]
}`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		// Create .security.yml
		securityPath := filepath.Join(tmpDir, SecurityConfigFile)
		securityContent := `model_list:
  multi-key-model:0:
    api_key: "sk-key-1"
    api_keys:
      - "sk-key-1"
      - "sk-key-2"
      - "sk-key-3"
`
		err = os.WriteFile(securityPath, []byte(securityContent), 0o600)
		require.NoError(t, err)

		// Load config
		cfg, err := LoadConfig(configPath)
		require.NoError(t, err)

		t.Logf("Config: %+v", cfg.ModelList)
		for _, m := range cfg.ModelList {
			t.Logf("Model: %+v", m)
		}
		// Verify multi-key expansion works
		assert.Equal(t, 3, len(cfg.ModelList))
		assert.Equal(t, "multi-key-model", cfg.ModelList[2].ModelName)
	})
}
