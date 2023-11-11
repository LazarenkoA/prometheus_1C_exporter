package main

import (
	"fmt"
	"io/ioutil"
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
	cmd := exec.Command("git", "tag", "-af", newversion, fmt.Sprintf("-m %s", newversion))
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Run(); err != nil {
		txt, _ := ioutil.ReadAll(stderr)
		fmt.Printf("Произошла ошибка: %v\n\tout: %s\n", err, string(txt))
	}
	exec.Command("git", "push", "--tags").Run()

	return
}
