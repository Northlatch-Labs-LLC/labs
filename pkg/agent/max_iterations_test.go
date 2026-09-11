package agent

import (
	"testing"

	"github.com/Northlatch-Labs-LLC/labs/pkg/config"
)

func TestEffectiveMaxToolIterations(t *testing.T) {
	cases := []struct {
		name       string
		configured int
		want       int
	}{
		{"unset takes the default", 0, config.DefaultMaxToolIterations},
		{"negative takes the default", -3, config.DefaultMaxToolIterations},
		{"within bounds is kept", 5, 5},
		{"exactly the ceiling is kept", config.MaxToolIterationsCeiling, config.MaxToolIterationsCeiling},
		{"upstream default 50 is clamped", 50, config.MaxToolIterationsCeiling},
		{"absurd value is clamped", 1000, config.MaxToolIterationsCeiling},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveMaxToolIterations(tc.configured); got != tc.want {
				t.Fatalf("effectiveMaxToolIterations(%d) = %d, want %d", tc.configured, got, tc.want)
			}
		})
	}
	if config.DefaultMaxToolIterations != 8 || config.MaxToolIterationsCeiling != 20 {
		t.Fatalf("bounds changed: default %d ceiling %d; the commit that changes them must say why",
			config.DefaultMaxToolIterations, config.MaxToolIterationsCeiling)
	}
}
