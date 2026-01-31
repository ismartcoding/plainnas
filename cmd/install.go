package cmd

import (
	"ismartcoding/plainnas/cmd/install"

	"github.com/spf13/cobra"
)

var installWithLibreOffice bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install related packages on server and setup environment.",
	Run: func(cmd *cobra.Command, args []string) {
		install.Install(install.Options{WithLibreOffice: installWithLibreOffice})
	},
}

func init() {
	installCmd.Flags().BoolVar(&installWithLibreOffice, "with-libreoffice", false, "Install LibreOffice for DOC/DOCX PDF preview")
}
