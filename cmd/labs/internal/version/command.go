package version

import (
	"github.com/spf13/cobra"

	"github.com/Northlatch-Labs-LLC/labs/cmd/labs/internal"
	"github.com/Northlatch-Labs-LLC/labs/cmd/labs/internal/cliui"
	"github.com/Northlatch-Labs-LLC/labs/pkg/config"
)

func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			printVersion()
		},
	}

	return cmd
}

func printVersion() {
	build, goVer := config.FormatBuildInfo()
	cliui.PrintVersion(internal.Logo, "labs "+config.FormatVersion(), build, goVer)
}
