package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type BaseExplorer struct {
	summary     *prometheus.SummaryVec
	timerNotyfy time.Duration
}

func (this *BaseExplorer) run(cmd *exec.Cmd) (string, error) {
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	err := cmd.Run()
	stderr := cmd.Stderr.(*bytes.Buffer).String()
	if err != nil {
		errText := fmt.Sprintf("Произошла ошибка запуска:\n err:%v \n", string(err.Error()))
		if stderr != "" {
			errText += fmt.Sprintf("StdErr:%v \n", stderr)
		}
		return "", errors.New(errText)
	}
	return cmd.Stdout.(*bytes.Buffer).String(), err
}
