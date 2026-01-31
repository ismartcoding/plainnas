package install

import (
	_ "embed"
	"errors"
	"ismartcoding/plainnas/internal/consts"
	"log"
	"os"
	"regexp"

	"github.com/google/uuid"
)

//go:embed plainnas.service
var plainNasService []byte

//go:embed config.toml
var configToml []byte

// Removed worker service and build script embedding (no C worker).

type Options struct {
	WithLibreOffice bool
}

func Install(opts Options) {
	// Installer output should be easy to read.
	log.SetFlags(0)

	installAllDeps()
	if opts.WithLibreOffice {
		installLibreOffice()
	} else {
		printNote("DOC/DOCX preview (LibreOffice)", "Optional. Run: plainnas install --with-libreoffice")
	}
	runSelfChecks()

	// Ensure directories exist (systemd: 0755, app config: 0755)
	_ = os.MkdirAll("/etc/plainnas", 0755)

	// Professional permissions: service 0644, config 0640
	if err := os.WriteFile("/etc/systemd/system/plainnas.service", plainNasService, 0644); err != nil {
		log.Println(err)
	}

	// Do not overwrite user config. Only write default config on first install.
	if _, err := os.Stat(consts.ETC_MAIN_CONFIG); err == nil {
		printOK("Config", "Keeping existing "+consts.ETC_MAIN_CONFIG)
	} else if errors.Is(err, os.ErrNotExist) {
		// Replace id in configToml with a new uuid before writing
		newID := uuid.NewString()
		re := regexp.MustCompile(`id = ".*"`)
		configStr := string(configToml)
		configStr = re.ReplaceAllString(configStr, "id = \""+newID+"\"")
		if err := os.WriteFile(consts.ETC_MAIN_CONFIG, []byte(configStr), 0640); err != nil {
			log.Println(err)
		} else {
			printOK("Config", "Written "+consts.ETC_MAIN_CONFIG)
		}
	} else {
		log.Println(err)
	}
}
