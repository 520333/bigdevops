package common

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(logLevel string, logFilePath string) *zap.Logger {
	// 日志级别
	atomicLevel := zap.NewAtomicLevel()
	switch strings.ToLower(strings.TrimSpace(logLevel)) {
	case "debug":
		atomicLevel.SetLevel(zap.DebugLevel)
	case "info":
		atomicLevel.SetLevel(zap.InfoLevel)
	case "warn", "warning":
		atomicLevel.SetLevel(zap.WarnLevel)
	case "error":
		atomicLevel.SetLevel(zap.ErrorLevel)
	case "dpanic":
		atomicLevel.SetLevel(zap.DPanicLevel)
	case "panic":
		atomicLevel.SetLevel(zap.PanicLevel)
	case "fatal":
		atomicLevel.SetLevel(zap.FatalLevel)
	default:
		atomicLevel.SetLevel(zap.InfoLevel)
	}
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "name",
		CallerKey:      "line",
		MessageKey:     "msg",
		FunctionKey:    "func",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,                              // 换行符
		EncodeLevel:    zapcore.LowercaseLevelEncoder,                          // 小写
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05:000"), // 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
	writer := &lumberjack.Logger{
		Filename:   logFilePath, // 日志名称
		MaxSize:    100,         // 日志大小限制 100mb
		MaxAge:     30,          // 历史日志文件保存天数
		MaxBackups: 10,          // 最大保留历史日志数量
		LocalTime:  true,        // 本地时区
		Compress:   false,       // 历史日志文件压缩标识
	}
	zapCoreFile := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(writer), atomicLevel)

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	zapCoreConsole := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), atomicLevel)
	core := zapcore.NewTee(zapCoreFile, zapCoreConsole)

	return zap.New(core, zap.AddCaller())

}
