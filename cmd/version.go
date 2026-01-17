package cmd

import "fmt"

var (
	Version   = "0.1.0"
	BuildTime = "dev"
	GitCommit = "dev"
)

func FullVersion() string {
	return fmt.Sprintf("PlainNAS v%s (built %s, commit %s)",
		Version, BuildTime, GitCommit)
}
