package cmd

import (
	"context"
	"fmt"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/update"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Self-update helpers (download/apply via updater)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var updateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check latest GitHub release",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		r, err := update.DownloadLatestRelease(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("latest=%s url=%s\n", strings.TrimSpace(r.TagName), strings.TrimSpace(r.HTMLURL))
		return nil
	},
}

var updateDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download + verify + write <binary>.new (does not replace running binary)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()

		binaryPath, err := os.Executable()
		if err != nil {
			return err
		}

		tag, _ := cmd.Flags().GetString("tag")
		if strings.TrimSpace(tag) == "" {
			r, err := update.DownloadLatestRelease(ctx)
			if err != nil {
				return err
			}
			tag = strings.TrimSpace(r.TagName)
		}

		res, err := update.DownloadAndPrepare(ctx, tag, binaryPath)
		if err != nil {
			return err
		}
		fmt.Printf("prepared %s\n", res.NewBinPath)
		return nil
	},
}

var updateApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply <binary>.new via plainnas-updater (stop/start with rollback)",
	RunE: func(cmd *cobra.Command, args []string) error {
		binaryPath, err := os.Executable()
		if err != nil {
			return err
		}
		newPath, err := update.EnsureSameDirNewPath(binaryPath)
		if err != nil {
			return err
		}
		oldPath := update.EnsureOldPath(binaryPath)

		updaterPath, _ := cmd.Flags().GetString("updater")
		if strings.TrimSpace(updaterPath) == "" {
			dir := filepath.Dir(binaryPath)
			updaterPath = filepath.Join(dir, "plainnas-updater")
		}

		serviceName, _ := cmd.Flags().GetString("service")
		if strings.TrimSpace(serviceName) == "" {
			serviceName = "plainnas"
		}

		healthURL := inferHealthURL()

		plan := update.Plan{
			ServiceName: serviceName,
			BinaryPath:  binaryPath,
			NewPath:     newPath,
			OldPath:     oldPath,
			HealthURL:   healthURL,
		}
		return update.RunUpdaterViaSystemdRun(updaterPath, plan)
	},
}

func inferHealthURL() string {
	cfg := config.GetDefault()
	port := strings.TrimSpace(cfg.GetString("server.http_port"))
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf("http://127.0.0.1:%s%s", port, update.DefaultHealthPath)
}

func init() {
	updateDownloadCmd.Flags().String("tag", "", "release tag (default: latest)")

	updateApplyCmd.Flags().String("service", "plainnas", "systemd service name")
	updateApplyCmd.Flags().String("updater", "", "path to plainnas-updater (default: alongside main binary)")

	updateCmd.AddCommand(updateCheckCmd)
	updateCmd.AddCommand(updateDownloadCmd)
	updateCmd.AddCommand(updateApplyCmd)
}
