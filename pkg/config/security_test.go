// Labs - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Labs contributors

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityPath(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		want      string
	}{
		{
			name:      "standard path",
			configDir: "/home/user/.labs/config.json",
			want:      "/home/user/.labs/.security.yml",
		},
		{
			name:      "nested path",
			configDir: "/path/to/config/myconfig.json",
			want:      "/path/to/config/.security.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := securityPath(tt.configDir)
			assert.Equal(t, tt.want, got)
		})
	}
}
