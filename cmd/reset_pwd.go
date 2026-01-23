package cmd

import (
	"fmt"
	"ismartcoding/plainnas/internal/db"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var resetPwdCmd = &cobra.Command{
	Use:   "passwd",
	Short: "Set or reset admin password",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for root privileges
		if syscall.Getuid() != 0 {
			return fmt.Errorf("this command requires root privileges; run with sudo")
		}

		fromStdin, _ := cmd.Flags().GetBool("stdin")
		var pwd string
		if fromStdin {
			b, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return err
			}
			pwd = strings.TrimSpace(string(b))
			if pwd == "" {
				return fmt.Errorf("password cannot be empty")
			}
		} else {
			p, err := promptNewPassword()
			if err != nil {
				return err
			}
			pwd = p
		}

		if err := db.SetAdminPassword(pwd); err != nil {
			return err
		}
		fmt.Println("admin password updated")
		return nil
	},
}

func init() {
	resetPwdCmd.Flags().Bool("stdin", false, "read password from stdin (single line, no prompt)")
}
