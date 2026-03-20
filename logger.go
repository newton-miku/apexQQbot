package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// FileLogger file logger
type FileLogger struct {
	logger *zap.Logger
	sugar  *zap.SugaredLogger
	file   *os.File
}

type logLevel zapcore.Level

const (
	DebugLevel = logLevel(zapcore.DebugLevel)
	InfoLevel  = logLevel(zapcore.InfoLevel)
	WarnLevel  = logLevel(zapcore.WarnLevel)
	FatalLevel = logLevel(zapcore.FatalLevel)
)

// 全局 logger 实例
var globalLogger *FileLogger

// New creates a new FileLogger with timestamped filename.
func New(logPath string, minLogLevel logLevel) (FileLogger, error) {
	// 使用 log 子文件夹
	logDir := filepath.Join(logPath, "log")
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return FileLogger{}, err
	}

	// 生成带时间戳的日志文件名：apexQQbot_YYYY-MM-DD_HH-MM-SS.log
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(logDir, fmt.Sprintf("apexQQbot_%s.log", timestamp))
	latestLink := filepath.Join(logDir, "apexQQbot_latest.log")

	// 以追加模式打开文件，如果不存在则创建
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return FileLogger{}, err
	}

	// 创建/更新 "latest" 符号链接
	_ = os.Remove(latestLink)
	if err := os.Symlink(filepath.Base(logFile), latestLink); err != nil {
		// 如果符号链接创建失败（Windows 可能需要管理员权限），则复制文件
		copyFile(logFile, latestLink)
	}

	// 配置 zap encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 创建多个输出：文件 + 控制台
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	fileEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.Level(minLogLevel)),
		zapcore.NewCore(fileEncoder, zapcore.AddSync(file), zapcore.Level(minLogLevel)),
	)

	logger := zap.New(core, zap.AddCallerSkip(2))
	sugar := logger.Sugar()

	fileLogger := FileLogger{
		logger: logger,
		sugar:  sugar,
		file:   file,
	}

	// 设置全局 logger
	globalLogger = &fileLogger

	// 将标准库 log 重定向到我们的 logger
	multiWriter := io.MultiWriter(file, os.Stdout)
	log.SetOutput(multiWriter)
	log.SetFlags(0)

	return fileLogger, nil
}

// copyFile 复制文件作为创建符号链接的备选方案
func copyFile(src, dst string) {
	source, err := os.Open(src)
	if err != nil {
		return
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return
	}
	defer destination.Close()

	_, _ = io.Copy(destination, source)
}

// GetGlobalLogger 获取全局 logger 实例
func GetGlobalLogger() *FileLogger {
	return globalLogger
}

// GetWriter 获取日志写入器，用于重定向其他日志
func (f FileLogger) GetWriter() io.Writer {
	if f.file != nil {
		return io.MultiWriter(f.file, os.Stdout)
	}
	return os.Stdout
}

// Debug logs a message at DebugLevel.
func (f FileLogger) Debug(v ...any) {
	f.logger.Debug(output(v...))
}

// Info logs a message at InfoLevel.
func (f FileLogger) Info(v ...any) {
	f.logger.Info(output(v...))
}

// Warn logs a message at WarnLevel.
func (f FileLogger) Warn(v ...any) {
	f.logger.Warn(output(v...))
}

// Error logs a message at ErrorLevel.
func (f FileLogger) Error(v ...any) {
	f.logger.Error(output(v...))
}

// Debugf logs a message at DebugLevel.
func (f FileLogger) Debugf(format string, v ...any) {
	f.logger.Debug(output(fmt.Sprintf(format, v...)))
}

// Infof logs a message at InfoLevel.
func (f FileLogger) Infof(format string, v ...any) {
	f.logger.Info(output(fmt.Sprintf(format, v...)))
}

// Warnf logs a message at WarnLevel.
func (f FileLogger) Warnf(format string, v ...any) {
	f.logger.Warn(output(fmt.Sprintf(format, v...)))
}

// Errorf logs a message at ErrorLevel.
func (f FileLogger) Errorf(format string, v ...any) {
	f.logger.Error(output(fmt.Sprintf(format, v...)))
}

// Printf implements botgo.Logger interface.
func (f FileLogger) Printf(format string, v ...any) {
	f.logger.Info(output(fmt.Sprintf(format, v...)))
}

// Print implements botgo.Logger interface.
func (f FileLogger) Print(v ...any) {
	f.logger.Info(output(fmt.Sprint(v...)))
}

// Println implements botgo.Logger interface.
func (f FileLogger) Println(v ...any) {
	f.logger.Info(output(fmt.Sprintln(v...)))
}

// Sync flushes any buffered log entries.
func (f FileLogger) Sync() error {
	return f.logger.Sync()
}

// Close 关闭日志文件
func (f FileLogger) Close() error {
	if f.file != nil {
		_ = f.Sync()
		return f.file.Close()
	}
	return nil
}

// Fatal logs a message at FatalLevel and then calls os.Exit(1)
func (f FileLogger) Fatal(v ...any) {
	f.logger.Error(output(v...))
	_ = f.Sync()
	os.Exit(1)
}

// Fatalf logs a message at FatalLevel with format and then calls os.Exit(1)
func (f FileLogger) Fatalf(format string, v ...any) {
	f.logger.Error(output(fmt.Sprintf(format, v...)))
	_ = f.Sync()
	os.Exit(1)
}

func output(v ...any) string {
	pc, file, line, _ := runtime.Caller(3)
	file = filepath.Base(file)
	funcName := strings.TrimPrefix(filepath.Ext(runtime.FuncForPC(pc).Name()), ".")

	logFormat := "%s:%d:%s " + fmt.Sprint(v...)
	return fmt.Sprintf(logFormat, file, line, funcName)
}
