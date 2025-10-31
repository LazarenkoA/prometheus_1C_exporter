package logger

import (
	"os"
	"testing"
)

func TestNewLogger(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Проверяем обычный режим (isTesting == false)
	os.Args = []string{"app", "run"}
	log := newLogger(os.TempDir())
	if log == nil {
		t.Error("newLogger() should not return nil")
	}
	log.Info("test log (not in testing mode)")

	// Проверяем режим тестирования (isTesting == true)
	os.Args = []string{"app", "-test.v"}
	logTest := newLogger(os.TempDir())
	if logTest == nil {
		t.Error("newLogger() should not return nil in testing mode")
	}
	logTest.Info("test log (in testing mode)")
}