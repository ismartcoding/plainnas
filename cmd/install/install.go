package install

import (
	_ "embed"
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

func Install() {
	// Installer output should be easy to read.
	log.SetFlags(0)

	installAllDeps()
	runSelfChecks()

	// Ensure directories exist (systemd: 0755, app config: 0755)
	_ = os.MkdirAll("/etc/plainnas", 0755)

	// Professional permissions: service 0644, config 0640
	if err := os.WriteFile("/etc/systemd/system/plainnas.service", plainNasService, 0644); err != nil {
		log.Println(err)
	}

	// Replace id in configToml with a new uuid before writing
	newID := uuid.NewString()
	re := regexp.MustCompile(`id = ".*"`)
	configStr := string(configToml)
	configStr = re.ReplaceAllString(configStr, "id = \""+newID+"\"")
	if err := os.WriteFile(consts.ETC_MAIN_CONFIG, []byte(configStr), 0640); err != nil {
		log.Println(err)
	}
}
