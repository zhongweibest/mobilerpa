package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

const (
	defaultRetentionDays = 7
	logFlags             = log.LstdFlags | log.Lmicroseconds
)

var (
	mu              sync.Mutex
	appCloser       io.Closer
	wsCloser        io.Closer
	schedulerCloser io.Closer
	errorCloser     io.Closer
	wsLogger        = log.New(os.Stdout, "[ws] ", logFlags)
	schedulerLogger = log.New(os.Stdout, "[scheduler] ", logFlags)
	errorLogger     = log.New(os.Stderr, "[error] ", logFlags)
)

// Setup 初始化中心服务的日志输出。
func Setup(rootPath string, retentionDays int) error {
	rootPath = filepath.Clean(rootPath)
	if rootPath == "" || rootPath == "." {
		rootPath = "./data/logs"
	}
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}

	serverDir := filepath.Join(rootPath, "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	appWriter, appRotator, err := newRotatingWriter(serverDir, "app", retentionDays)
	if err != nil {
		return err
	}
	wsWriter, wsRotator, err := newRotatingWriter(serverDir, "ws", retentionDays)
	if err != nil {
		_ = appRotator.Close()
		return err
	}
	schedulerWriter, schedulerRotator, err := newRotatingWriter(serverDir, "scheduler", retentionDays)
	if err != nil {
		_ = appRotator.Close()
		_ = wsRotator.Close()
		return err
	}
	errorWriter, errorRotator, err := newRotatingWriter(serverDir, "error", retentionDays)
	if err != nil {
		_ = appRotator.Close()
		_ = wsRotator.Close()
		_ = schedulerRotator.Close()
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	closeIfPresent(appCloser)
	closeIfPresent(wsCloser)
	closeIfPresent(schedulerCloser)
	closeIfPresent(errorCloser)

	appCloser = appRotator
	wsCloser = wsRotator
	schedulerCloser = schedulerRotator
	errorCloser = errorRotator

	log.SetFlags(logFlags)
	log.SetPrefix("[app] ")
	log.SetOutput(io.MultiWriter(os.Stdout, appWriter))

	wsLogger = log.New(io.MultiWriter(os.Stdout, wsWriter), "[ws] ", logFlags)
	schedulerLogger = log.New(io.MultiWriter(os.Stdout, schedulerWriter), "[scheduler] ", logFlags)
	errorLogger = log.New(io.MultiWriter(os.Stderr, errorWriter), "[error] ", logFlags)

	return nil
}

// WSF 向 WebSocket 日志输出普通信息。
func WSF(format string, args ...any) {
	wsLogger.Printf(format, args...)
}

// WSErrorf 向 WebSocket 日志和错误日志同时输出错误信息。
func WSErrorf(format string, args ...any) {
	writeError(wsLogger, "ws", format, args...)
}

// Schedulerf 向调度器日志输出普通信息。
func Schedulerf(format string, args ...any) {
	schedulerLogger.Printf(format, args...)
}

// SchedulerErrorf 向调度器日志和错误日志同时输出错误信息。
func SchedulerErrorf(format string, args ...any) {
	writeError(schedulerLogger, "scheduler", format, args...)
}

// AppErrorf 向应用日志和错误日志同时输出错误信息。
func AppErrorf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	log.Printf("ERROR %s", message)
	errorLogger.Printf("[app] %s", message)
}

func writeError(target *log.Logger, scope string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	target.Printf("ERROR %s", message)
	errorLogger.Printf("[%s] %s", scope, message)
}

func newRotatingWriter(dir string, name string, retentionDays int) (io.Writer, io.Closer, error) {
	pattern := filepath.Join(dir, name+".%Y%m%d.log")
	writer, err := rotatelogs.New(
		pattern,
		rotatelogs.WithClock(rotatelogs.Local),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithMaxAge(time.Duration(retentionDays)*24*time.Hour),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s logger: %w", name, err)
	}
	return writer, writer, nil
}

func closeIfPresent(closer io.Closer) {
	if closer == nil {
		return
	}
	_ = closer.Close()
}
