package server

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"dst/internal/bitrate"
)

func generateRequestId() (string, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "rnd-failed", err
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

func RunServer(port int, b bitrate.Bitrate, randomBytes int) error {
	slog.Info(fmt.Sprintf("Listen for connection at :%d", port))

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId, err := generateRequestId()

		l := slog.Default().With(slog.String("request_id", requestId))
		l.Debug("Start responding to a new request")

		if err != nil {
			l.Error("Failed to generate random request ID: " + err.Error())
		}

		var in io.Reader
		if randomBytes == 0 {
			in = bufio.NewReaderSize(rand.Reader, 16<<10)
		} else {
			in, err = makeCycleRandomReader(randomBytes, 16<<10)
			if err != nil {
				l.Error("Failed to make random reader: " + err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// Ignore actual offset requested, I mean we provide random bytes anyway
		w.Header().Set("Accept-Ranges", "bytes")

		var n int64
		if b == 0 {
			n, err = io.Copy(w, in)
		} else {
			t := time.NewTicker(time.Second / 24)
			stopC := make(chan struct{})
			rndC, errC := runGen(in, int(b)/24, stopC, l)
			in := bytes.NewReader(nil)
			flushAt := time.Now().Add(time.Second)
		outer:
			for {
				select {
				case <-errC:
					return
				case <-t.C:
					var buf []byte
					select {
					case <-errC:
						return
					case buf = <-rndC:
					}

					in.Reset(buf)
					var n_ int64
					n_, err = io.Copy(w, in)
					n += n_
					if err != nil {
						t.Stop()
						close(stopC)
						break outer
					}

					if time.Now().After(flushAt) {
						flushAt = time.Now().Add(time.Second)
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
					}
				}
			}
		}

		if err != nil {
			l.Error("Error writing response: "+err.Error(), slog.Int64("bytes_written", n))
		} else {
			l.Debug("Finished responding to the request", slog.Int64("bytes_written", n))
		}
	}))

	if err != nil {
		slog.Error("Server stopped because of error: " + err.Error())
		return err
	}

	return nil
}

type looper struct {
	bs     []byte
	offset int
}

func (l *looper) Read(p []byte) (n int, err error) {
	for len(p) > 0 {
		copyLen := len(p)
		if copyLen+l.offset > len(l.bs) {
			copyLen = len(l.bs) - l.offset
		}

		copy(p[:copyLen], l.bs[l.offset:])
		l.offset += copyLen
		l.offset %= len(l.bs)
		p = p[copyLen:]

		n += copyLen
	}

	return
}

func makeCycleRandomReader(randomBytes int, minBufSize int) (io.Reader, error) {
	rnd := make([]byte, randomBytes)
	if _, err := io.ReadFull(rand.Reader, rnd); err != nil {
		return nil, err
	}

	bufSize := minBufSize
	if rem := bufSize % randomBytes; rem != 0 {
		bufSize += randomBytes - rem
	}
	bs := make([]byte, bufSize)
	_, err := io.ReadFull(&looper{
		bs: rnd,
	}, bs)
	if err != nil {
		panic("Impossible error from looper.Read: " + err.Error())
	}

	return &looper{bs: bs}, nil
}

func runGen(r io.Reader, bufSize int, stopC chan struct{}, l *slog.Logger) (chan []byte, chan error) {
	buf := make([]byte, bufSize)
	bufBack := make([]byte, bufSize)

	c := make(chan []byte)
	e := make(chan error, 1)
	go func() {
		for {
			_, err := io.ReadFull(r, buf)
			if err != nil {
				l.Error("Error reading from random reader: " + err.Error())
				e <- err
				return
			}

			select {
			case <-stopC:
				return
			case c <- buf:
			}

			buf, bufBack = bufBack, buf
		}
	}()

	return c, e
}
