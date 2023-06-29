package explorer

import (
	"context"
	"testing"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/pkg/errors"
)

func Test_fillBaseList(t *testing.T) {
	objectCSJ := new(ExplorerCheckSheduleJob)
	objectCSJ.ctx, objectCSJ.cancel = context.WithCancel(context.Background())
	objectCSJ.logger = logger.NopLogger.Named("test")

	t.Run("error", func(t *testing.T) {
		delay := time.NewTicker(time.Second * 200)
		p := gomonkey.ApplyFunc(time.NewTicker, func(d time.Duration) *time.Ticker { return delay })
		defer p.Reset()

		p.ApplyPrivateMethod(objectCSJ, "getListInfobase", func(_ *ExplorerCheckSheduleJob) error {
			return errors.New("error")
		})

		go objectCSJ.fillBaseList()

		time.Sleep(time.Second)
		objectCSJ.cancel()
	})

	t.Run("pass", func(t *testing.T) {
		delay := time.NewTicker(time.Millisecond * 200)
		p := gomonkey.ApplyFunc(time.NewTicker, func(d time.Duration) *time.Ticker { return delay })
		defer p.Reset()

		p.ApplyPrivateMethod(objectCSJ, "getListInfobase", func(_ *ExplorerCheckSheduleJob) error {
			return nil
		})

		go objectCSJ.fillBaseList()

		time.Sleep(time.Second)
		objectCSJ.cancel()
	})

}
