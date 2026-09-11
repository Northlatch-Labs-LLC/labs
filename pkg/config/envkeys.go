// Labs - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Labs contributors

package config

import (
	"os"
	"path/filepath"

	"github.com/Northlatch-Labs-LLC/labs/pkg"
)

// Runtime environment variable keys for the labs process.
// These control the location of files and binaries at runtime and are read
// directly via os.Getenv / os.LookupEnv. All labs-specific keys use the
// LABS_ prefix. Reference these constants instead of inline string
// literals to keep all supported knobs visible in one place and to prevent
// typos.
const (
	// EnvHome overrides the base directory for all labs data
	// (config, workspace, skills, auth store, …).
	// Default: ~/.labs
	EnvHome = "LABS_HOME"

	// EnvConfig overrides the full path to the JSON config file.
	// Default: $LABS_HOME/config.json
	EnvConfig = "LABS_CONFIG"

	// EnvBuiltinSkills overrides the directory from which built-in
	// skills are loaded.
	// Default: <cwd>/skills
	EnvBuiltinSkills = "LABS_BUILTIN_SKILLS"

	// EnvBinary overrides the path to the labs executable.
	// Used by the web launcher when spawning the gateway subprocess.
	// Default: resolved from the same directory as the current executable.
	EnvBinary = "LABS_BINARY"

	// EnvGatewayHost overrides the host address for the gateway server.
	// Default: "localhost"
	EnvGatewayHost = "LABS_GATEWAY_HOST"
)

func GetHome() string {
	homePath, _ := os.UserHomeDir()
	if labsHome := os.Getenv(EnvHome); labsHome != "" {
		homePath = labsHome
	} else if homePath != "" {
		homePath = filepath.Join(homePath, pkg.DefaultLabsHome)
	}
	if homePath == "" {
		homePath = "."
	}
	return homePath
}
