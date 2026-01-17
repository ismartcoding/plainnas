package cmd

import (
	"ismartcoding/plainnas/cmd/install"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install related packages on server and setup environment.",
	Run: func(cmd *cobra.Command, args []string) {
		install.Install()
	},
}
