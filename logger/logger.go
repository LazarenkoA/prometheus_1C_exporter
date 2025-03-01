package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger *zap.SugaredLogger
	atom   zap.AtomicLevel
)

var levelMap = map[int]zapcore.Level{
	5: zapcore.DebugLevel,
	4: zapcore.InfoLevel,
	3: zapcore.WarnLevel,
	2: zapcore.ErrorLevel,
}

var (
	DefaultLogger *zap.SugaredLogger
	NopLogger     *zap.SugaredLogger
)

func init() {
	atom = zap.NewAtomicLevel()
	NopLogger = newNopLogger()
	DefaultLogger = newLogger(filepath.Join("", "logs"))
}

func InitLogger(logDir string, ll int) {
	DefaultLogger = newLogger(filepath.Join(logDir, "logs"))
	SetLevel(ll)
}

func newLogger(logDir string) *zap.SugaredLogger {
	var logWriter io.Writer

	if isTesting() {
		logWriter = os.Stdout
	} else {
		logWriter = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "log.txt"),
			MaxSize:    10, // megabytes
			MaxBackups: 10,
			MaxAge:     5, // days
		}
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(logWriter),
		atom,
	)

	return zap.New(core).Sugar()
}

func SetLevel(level int) {
	atom.SetLevel(levelMap[level])
}

func newNopLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

func isTesting() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test") {
			return true
		}
	}

	return false
}
