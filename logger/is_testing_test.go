package logger

import (
	"os"
	"testing"
)

func TestIsTesting(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "-test.v"}
	if !isTesting() {
		t.Error("isTesting() should return true when -test flag is present")
	}

	os.Args = []string{"app", "run"}
	if isTesting() {
		t.Error("isTesting() should return false when -test flag is absent")
	}
}