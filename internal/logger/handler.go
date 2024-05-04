package logger

import (
	"context"
	"go/build"
	"log/slog"
	"os"
	"runtime"
	"strings"
)

func getEnvOrDefault(key, default_ string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return default_
}

var (
	logFormat      = getEnvOrDefault("LOG_FORMAT", "text")
	logLevel       = strings.ToLower(getEnvOrDefault("LOG_LEVEL", "info"))
	defaultHandler *handler
)

// SetupSLog configures logging handler with format depending on environment var LOG_FORMAT
// and which strips common prefix from file paths (rootPath param)
func SetupSLog(rootPath string) {
	var lvl slog.Level
	switch logLevel {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		slog.Error("LOG_LEVEL must be one of: debug, info, warn, error")
		os.Exit(1)
	}

	ho := slog.HandlerOptions{
		Level: lvl,
	}

	var h slog.Handler
	switch logFormat {
	case "json":
		h = slog.NewJSONHandler(os.Stderr, &ho)
		break
	case "text":
		h = slog.NewTextHandler(os.Stderr, &ho)
		break
	default:
		slog.Error("LOG_FORMAT must be json or text")
		os.Exit(1)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	defaultHandler = &handler{
		baseHandler: h,
		rootPath:    strings.TrimSuffix(rootPath, "/") + "/",
		goPath:      strings.TrimSuffix(gopath, "/") + "/",
	}

	slog.SetDefault(slog.New(defaultHandler))
}

type handler struct {
	baseHandler slog.Handler
	rootPath    string
	goPath      string
}

func (e *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return e.baseHandler.Enabled(ctx, level)
}

func (e *handler) Handle(ctx context.Context, record slog.Record) error {
	record = record.Clone()

	hasSource := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == slog.SourceKey {
			hasSource = true
			return false
		}

		return true
	})

	if !hasSource && record.PC != 0 {
		record.AddAttrs(e.getSourceAttr(record.PC))
	}

	return e.baseHandler.Handle(ctx, record)
}

func (e *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{
		baseHandler: e.baseHandler.WithAttrs(attrs),
		rootPath:    e.rootPath,
		goPath:      e.goPath,
	}
}

func (e *handler) WithGroup(name string) slog.Handler {
	return &handler{
		baseHandler: e.baseHandler.WithGroup(name),
		rootPath:    e.rootPath,
		goPath:      e.goPath,
	}
}

func (e *handler) getSourceAttr(pc uintptr) slog.Attr {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	file := f.File
	if strings.HasPrefix(file, e.rootPath) {
		file = file[len(e.rootPath):]
	} else if strings.HasPrefix(file, e.goPath) {
		file = file[len(e.goPath):]
	}

	return slog.Any(slog.SourceKey, slog.Source{
		Function: f.Function,
		File:     file,
		Line:     f.Line,
	})
}

func GetSourceAttr(skipFrames int) slog.Attr {
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, skipFrames...]
	runtime.Callers(2+skipFrames, pcs[:])

	return defaultHandler.getSourceAttr(pcs[0])
}
