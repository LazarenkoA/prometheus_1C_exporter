package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
	}
}

func run() (err error) {
	lastversion, err := exec.Command("git", "describe", "--abbrev=0").Output()
	if err != nil {
		return
	}

	os.Setenv("PROM_VERSION", strings.Trim(string(lastversion), "\r\n"))

	return
}
