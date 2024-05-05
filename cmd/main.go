package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/alecthomas/kong"
	"golang.org/x/sync/errgroup"

	"dst/internal/bitrate"
	"dst/internal/logger"
	"dst/internal/player"
	"dst/internal/server"
)

type Tester struct {
	URL               *url.URL        `arg:"" env:"CONNECT_URL" help:"URL to connect to"`
	Bitrate           bitrate.Bitrate `required:"" short:"b" env:"BITRATE" help:"Target video emulated bitrate. Must be int with suffix of k, m or g, meaning kilobits, megabits and gigabits per second"`
	Threads           int             `short:"t" env:"NUM_THREADS" help:"Number of threads to use, each with a separate connection and consuming specified bitrate" default:"1"`
	BufferMin         int             `env:"BUFFER_MIN" help:"Keep buffering and NOT start playing until reached" default:"1"`
	BufferMax         int             `env:"BUFFER_MAX" help:"Stop buffering when reached" default:"10"`
	BufferToppedDelay int             `env:"BUFFER_TOPPED_DELAY" help:"When buffer is full, how long to wait before trying beginning to refill it again" default:"1"`
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

	if t.Threads < 1 {
		return fmt.Errorf("number of threads must be at least 1")
	}

	return nil
}

func (t *Tester) Run() error {
	if t.Threads == 1 {
		return t.run(slog.Default(), nil)
	}

	wg, ctx := errgroup.WithContext(context.Background())
	for i := range t.Threads {
		wg.Go(func() error {
			return t.run(slog.Default().With(slog.Int("thread", i)), ctx)
		})
	}

	return wg.Wait()
}

func (t *Tester) run(l *slog.Logger, ctx context.Context) error {
	b := player.NewBuffer(t.URL, t.Bitrate, t.BufferMin, t.BufferMax,
		time.Duration(t.BufferToppedDelay)*time.Second, ctx, l)
	p := player.NewEmulator(b)
	return p.Run()
}

type Server struct {
	Port        *int            `arg:"" env:"PORT" help:"Port to listen on"`
	Bitrate     bitrate.Bitrate `short:"b" env:"BITRATE" help:"Maximum bitrate for response, if desired. Must have suffix of k, m or g. By default bitrate is not artificially limited and depends only on your system CSPRNG and networking speed"`
	RandomBytes int             `env:"RANDOM_BYTES" help:"If set, only this number of random bytes will be generated, and then just cycled to produce output. Can be used to remove throughput dependency on CSPRNG generator performance"`
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

	if s.RandomBytes < 0 {
		return fmt.Errorf("random bytes must be positive")
	}

	return nil
}

func (s *Server) Run() error {
	return server.RunServer(*s.Port, s.Bitrate, s.RandomBytes)
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
