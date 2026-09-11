package channels

import (
	"context"
	"testing"

	"github.com/Northlatch-Labs-LLC/labs/pkg/commands"
)

type mockRegistrar struct{}

func (mockRegistrar) RegisterCommands(context.Context, []commands.Definition) error { return nil }

func TestCommandRegistrarCapable_Compiles(t *testing.T) {
	var _ CommandRegistrarCapable = mockRegistrar{}
}
