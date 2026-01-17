package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "plainnas",
	Version: FullVersion(),
	Short:   "Build your lightweight NAS.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

func Execute() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.Flags().BoolP("version", "v", false, "version")

	rootCmd.Execute()
}
