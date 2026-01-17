package install

import (
	"fmt"
	"log"
	"strings"
)

const (
	colorReset  = "\x1b[0m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorRed    = "\x1b[31m"
	colorCyan   = "\x1b[36m"
)

func printSection(title string) {
	log.Println(colorCyan + "\n== " + title + " ==" + colorReset)
}

func printOK(title, detail string) {
	printLine(colorGreen+"OK"+colorReset, title, detail)
}

func printNote(title, detail string) {
	printLine(colorYellow+"NOTE"+colorReset, title, detail)
}

func printFail(title, detail string) {
	printLine(colorRed+"FAIL"+colorReset, title, detail)
}

func printLine(status, title, detail string) {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		log.Println(fmt.Sprintf("[%s] %s", status, title))
		return
	}
	log.Println(fmt.Sprintf("[%s] %s: %s", status, title, detail))
}
