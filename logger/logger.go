package logger

import (
	"path/filepath"

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
)

func init() {
	atom = zap.NewAtomicLevel()
}

func InitLogger(logDir string, ll int) {
	DefaultLogger = newLogger(filepath.Join(logDir, "logs"))
	SetLevel(ll)
}

func newLogger(logDir string) *zap.SugaredLogger {
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(logDir, "log.txt"),
		MaxSize:    10, // megabytes
		MaxBackups: 10,
		MaxAge:     5, // days
	})

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		w,
		atom,
	)

	return zap.New(core).Sugar()
}

func SetLevel(level int) {
	atom.SetLevel(levelMap[level])
}
