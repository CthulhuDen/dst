package player

import (
	"io"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"dst/internal/bitrate"
	"dst/internal/downloader"
)

type Buffer struct {
	d                *downloader.Downloader
	br               bitrate.Bitrate
	minBuff, maxBuff int
	topBufDelay      time.Duration
	l                *slog.Logger

	lock   sync.Locker
	waitC  chan struct{}
	nBytes int
}

func NewBuffer(url *url.URL, br bitrate.Bitrate, minBuf, maxBuf int, topBufDelay time.Duration, l *slog.Logger) *Buffer {
	b := Buffer{
		br:          br,
		minBuff:     int(br) * minBuf,
		maxBuff:     int(br) * maxBuf,
		topBufDelay: topBufDelay,
		l:           l,
		lock:        &sync.Mutex{},
		waitC:       make(chan struct{}),
	}
	b.d = downloader.StartNewDownloader(url, b.handleNewBytes, l)
	l.Info("Starting filling buffer")
	return &b
}

func (b *Buffer) GetNextFrame(fps int) error {
	for {
		c := b.getBytesOrChannel(fps)
		if c == nil {
			return nil
		}

		b.l.Info("Cant play, wait while buffering")

		select {
		case <-c:
			b.l.Info("Continue playing")
		case <-b.d.WaitC():
			_, err := b.d.GetState()
			if err != nil {
				b.l.Error("Cant continue playing because download failed")
				return err
			}
			b.l.Info("Cant continue playing because end of file")
			return io.EOF
		}
	}
}

func (b *Buffer) getBytesOrChannel(fps int) chan struct{} {
	b.lock.Lock()
	defer b.lock.Unlock()

	needBytes := int(b.br) / fps
	if b.nBytes >= needBytes {
		b.nBytes -= needBytes
		return nil
	}

	if b.waitC == nil {
		b.waitC = make(chan struct{})
	}

	return b.waitC
}

func (b *Buffer) handleNewBytes(bs []byte) (needMore bool) {
	var waitC chan struct{}
	cont := true

	func() {
		b.lock.Lock()
		defer b.lock.Unlock()

		b.nBytes += len(bs)
		if b.waitC != nil && b.nBytes >= b.minBuff {
			waitC = b.waitC
			b.waitC = nil
		}
		if b.nBytes >= b.maxBuff {
			cont = false
		}
	}()

	if waitC != nil {
		close(waitC)
	}

	if !cont {
		b.l.Debug("Buffer is full, ask to pause download")

		go func() {
			time.Sleep(b.topBufDelay)

			if b.d.Resume() {
				b.l.Debug("Resuming download")
			} else {
				b.l.Warn("Wanted to resume download buy it is already active")
			}
		}()
	}

	return cont
}
