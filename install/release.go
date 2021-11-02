package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
	}
}

func run() (err error) {
	b_lastversion, err := exec.Command("git", "describe", "--abbrev=0").Output()
	if err != nil {
		fmt.Println("Произошла ошибка: ", err)
		return
	}

	lastversion := strings.Trim(string(b_lastversion), "\r\n")
	splitted := strings.Split(lastversion, ".")
	if len(splitted) < 3 {
		fmt.Println("последняя версия не коррректного формата")
		return
	}
	v, err := strconv.Atoi(splitted[len(splitted)-1])
	if err != nil {
		fmt.Println("Произошла ошибка: ", err)
		return
	}

	newversion := strings.Join(splitted[:len(splitted)-1], ".") + "." + strconv.Itoa(v+1)
	exec.Command("git", "tag", fmt.Sprintf("-af %s", newversion), fmt.Sprintf("-m %s", newversion)).Run()
	exec.Command("git", "push", "origin", "--tags").Run()

	return
}
