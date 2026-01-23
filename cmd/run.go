package cmd

import (
	"context"
	"ismartcoding/plainnas/cmd/services/api"
	"ismartcoding/plainnas/cmd/services/watcher"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/pkg/log"
	"ismartcoding/plainnas/internal/storage"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
)

var _context context.Context

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run plainnas service",
	Run: func(cmd *cobra.Command, args []string) {
		// Check for root privileges
		if syscall.Getuid() != 0 {
			println("This command requires root privileges. Please run with sudo.")
			os.Exit(1)
		}

		os.MkdirAll(consts.DATA_DIR, 0755)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		_context = ctx
		defer stop()

		config.Load()
		runtime.GOMAXPROCS(runtime.NumCPU())
		log.Init(config.GetDefault().GetString("log.level"))

		// Ensure a global URL token exists at startup
		db.EnsureURLToken()

		// Ensure tag relation secondary index exists for fast tag loading.
		if err := db.EnsureTagRelationKeyIndex(); err != nil {
			log.Errorf("tag relation index ensure failed: %v", err)
		}

		// Ensure media secondary indexes exist (type/trash/date) for fast listing/counting.
		if err := media.EnsureTypeIndexes(); err != nil {
			log.Errorf("media index ensure failed: %v", err)
		}

		// Scan and mount all discovered filesystems to /mnt/usbX based on
		// persisted FSUUID<->usbX mapping.
		if err := storage.EnsureMountedUSBVolumes(ctx); err != nil {
			log.Errorf("storage mount ensure failed: %v", err)
		}
		storage.RunAutoMountWatcher(ctx)

		go api.Run(ctx)
		go watcher.Run(ctx)

		<-ctx.Done()
		_ = db.GetDefault().Close()
	},
}
