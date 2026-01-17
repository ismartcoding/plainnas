package main

import (
	"ismartcoding/plainnas/cmd"

	"github.com/rs/zerolog"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cmd.Execute()
}
