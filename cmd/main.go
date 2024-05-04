package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/alecthomas/kong"

	"dst/internal/bitrate"
	"dst/internal/logger"
	"dst/internal/player"
	"dst/internal/server"
)

type Tester struct {
	URL               *url.URL        `arg:"" help:"URL to connect to"`
	Bitrate           bitrate.Bitrate `required:"" short:"b" help:"Target video emulated bitrate. Must be int with suffix of kb or mb, meaning kilobits and megabits per second"`
	BufferMin         int             `help:"Keep buffering and NOT start playing until reached" default:"1" placeholder:"SECONDS"`
	BufferMax         int             `help:"Stop buffering when reached" default:"10" placeholder:"SECONDS"`
	BufferToppedDelay int             `help:"When buffer is full, how long to wait before trying beginning to refill it again" default:"1" placeholder:"SECONDS"`
}

func (t *Tester) Validate() error {
	if t.BufferMin <= 0 {
		return fmt.Errorf("minimal buffer duration must be positive")
	}

	if t.BufferMax < t.BufferMin {
		return fmt.Errorf("maximal buffer duration must be greater than minimal buffer duration")
	}

	if t.BufferToppedDelay < 0 {
		return fmt.Errorf("buffer topped delay must be positive")
	}

	if t.BufferToppedDelay > t.BufferMax {
		return fmt.Errorf("buffer topped delay must be less than buffer max duration")
	}

	return nil
}

func (t *Tester) Run() error {
	b := player.NewBuffer(t.URL, t.Bitrate, t.BufferMin, t.BufferMax, time.Duration(t.BufferToppedDelay)*time.Second,
		slog.Default())
	p := player.NewEmulator(b)

	return p.Run()
}

type Server struct {
	Port    *int            `arg:"" help:"Port to listen on"`
	Bitrate bitrate.Bitrate `short:"b" help:"Maximum bitrate for response, if desired. Must have suffix of kb or mb"`
}

func (s *Server) Validate() error {
	if s.Port == nil {
		return nil
	}

	if *s.Port <= 0 {
		return fmt.Errorf("port must be positive")
	}

	if *s.Port > 65535 {
		return fmt.Errorf("port must be less than 65536")
	}

	return nil
}

func (s *Server) Run() error {
	return server.RunServer(*s.Port, s.Bitrate)
}

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	logger.SetupSLog(path.Dir(path.Dir(thisFile)))

	var cli struct {
		Tester *Tester `cmd:"" default:"withargs" help:"Emulate video streaming at given bitrate to stress test you internet connection to given URL"`
		Server *Server `cmd:"" help:"Run server which outputs random bytes to any connecting client"`
	}

	ctx := kong.Parse(&cli,
		kong.Name("dst"),
		kong.Description("Download Stress Tester - emulate video streaming at given bitrate to stress test you internet connection to given URL"),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
