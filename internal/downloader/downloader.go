package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"dst/internal/logger"
)

// Consumer receives downloaded bytes and must return whether more data
// is needed right now or not. Even when returning false it will not interrupt current download,
// merely will not attempt to read download buffer until resume.
//
// The function MUST be very fast, so nothing async please, and minimum allocations.
type Consumer = func([]byte) (needMore bool)

type remoteInfo struct {
	rangesSupported bool
	contentLength   int64
}

// Downloader is simple single-thread download client. It MUST NOT be copied.
// Safe for concurrent use from different goroutines.
type Downloader struct {
	consumer Consumer
	logger   *slog.Logger

	client     *http.Client
	ctx        context.Context
	req        *http.Request
	remoteInfo *remoteInfo

	respBody       io.ReadCloser
	buf            []byte
	consumedLength int64

	lock      sync.Locker
	closedC   chan struct{}
	isRunning bool
	err       error
	closed    bool
}

// StartNewDownloader will create new downloader instance and return it.
// It will initiate download process in the background, so consumer must be ready to handle
// incoming data immediately.
func StartNewDownloader(url *url.URL, consumer Consumer, ctx context.Context, logger *slog.Logger) *Downloader {
	if ctx == nil {
		ctx = context.Background()
	}

	if logger == nil {
		logger = slog.Default()
	}

	d := &Downloader{
		consumer: consumer,
		logger:   logger,
		client: &http.Client{
			// Copy definition of DefaultTransport, because I want dedicated connection pools for every client
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				// Align with our own buffer of 16K
				ReadBufferSize: 16 << 10,
			},
		},
		ctx: ctx,
		// Template request, only Ranges header may be changed before sending
		req: (&http.Request{
			Method:     "GET",
			URL:        url,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Host:       url.Host,
		}).Clone(ctx),
		buf:     make([]byte, 16<<10),
		lock:    &sync.Mutex{},
		closedC: make(chan struct{}),
	}

	d.isRunning = true
	go d.run()

	return d
}

// At most one thread can be running at any given time (use synchronization)
func (d *Downloader) run() {
	cont := true

	for cont {
		body := d.getResponseBody()
		if body == nil {
			return
		}

		var n int64
		var err error
		n, cont, err = d.readBody(body)
		d.consumedLength += n
		if err == io.EOF {
			body.Close()

			// only case when EOF is not the end of media - is we asked for range without knowing the full size of file
			if d.req.Header.Get("Range") != "" &&
				d.remoteInfo.contentLength < 0 &&
				n > 0 {

				continue
			}

			d.logger.Info("Download complete", slog.Int64("bytes", d.consumedLength))
			d.lockAndSaveFinished()
			return
		}
		if err != nil {
			body.Close()

			d.lockAndSetError(fmt.Errorf("error reading body: %v", err))
			return
		}

		if cont {
			panic("impossible: readBody returned without error (even EOF) and with positive cont")
		}

		d.respBody = body
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	d.logger.Debug("Pause download")

	d.isRunning = false
}

func (d *Downloader) readBody(body io.ReadCloser) (int64, bool, error) {
	var n int64 = 0

	for {
		var cont bool
		n_, err := body.Read(d.buf)
		if n_ > 0 {
			cont = d.consumer(d.buf[:n_])
		}
		n += int64(n_)

		if err != nil {
			return n, cont, err
		}

		if !cont {
			return n, false, nil
		}
	}
}

func (d *Downloader) lockAndSetError(err error) {
	d.logger.Error("Stopping downloader client because of error: "+err.Error(), logger.GetSourceAttr(1))

	d.lock.Lock()
	defer d.lock.Unlock()

	alreadyClosed := d.err != nil || d.closed

	d.isRunning = false
	d.err = err

	if !alreadyClosed {
		close(d.closedC)
	}
}

func (d *Downloader) lockAndSaveFinished() {
	d.lock.Lock()
	defer d.lock.Unlock()

	alreadyClosed := d.closed || d.err != nil

	d.isRunning = false
	d.closed = true

	if !alreadyClosed {
		close(d.closedC)
	}
}

func (d *Downloader) getResponseBody() io.ReadCloser {
	if d.respBody != nil {
		d.logger.Debug("Continue downloading response body")

		body := d.respBody
		d.respBody = nil
		return body
	}

	range_ := ""

	if d.consumedLength > 0 {
		if !d.remoteInfo.rangesSupported {
			d.lockAndSetError(fmt.Errorf("cannot continue download because ranges are not supported"))
			return nil
		}

		if d.remoteInfo.contentLength >= 0 {
			if d.consumedLength >= d.remoteInfo.contentLength {
				d.lockAndSetError(fmt.Errorf("cannot continue download because already consumed all content"))
				return nil
			}

			range_ = fmt.Sprintf("%d-%d", d.consumedLength, d.remoteInfo.contentLength-1)
		} else {
			range_ = fmt.Sprintf("%d-%d", d.consumedLength, d.consumedLength+100*1048576)
		}
		d.req.Header.Set("Range", "bytes="+range_)
	}

	d.logger.Debug("Making request", slog.String("range", range_))

	resp, err := d.client.Do(d.req)
	if err != nil {
		d.lockAndSetError(err)
		return nil
	}

	d.remoteInfo = &remoteInfo{
		rangesSupported: resp.Header.Get("Accept-Ranges") == "bytes",
		contentLength:   resp.ContentLength,
	}

	if resp.StatusCode != 200 {
		d.lockAndSetError(fmt.Errorf("bad status code: %d", resp.StatusCode))
		return nil
	}

	d.logger.Debug("Got response", slog.Bool("ranges_supported", d.remoteInfo.rangesSupported))

	return resp.Body
}

func (d *Downloader) Resume() bool {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.isRunning || d.err != nil || d.closed {
		return false
	}

	d.isRunning = true
	go d.run()

	return true
}

func (d *Downloader) WaitC() chan struct{} {
	return d.closedC
}

func (d *Downloader) WaitComplete() error {
	<-d.WaitC()

	d.lock.Lock()
	defer d.lock.Unlock()

	return d.err
}

func (d *Downloader) GetState() (running bool, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.err != nil {
		return false, d.err
	}

	return !d.closed, nil
}
