# Download Stress Tester

Utility to emulate video consumption from specified URL. Can be used to run on multiple instances
to check how much concurrent viewers can be realistically supported by your video stream server.

Consult the commands documentation for parameters. Every argument and flag can be set via environment
variable, which is useful for running in container.

```
Usage: dst tester --bitrate=BITRATE <url> [flags]

Emulate video streaming at given bitrate to stress test you internet connection to given URL

Arguments:
  <url>    URL to connect to ($CONNECT_URL)

Flags:
  -h, --help                     Show context-sensitive help.

  -b, --bitrate=BITRATE          Target video emulated bitrate. Must be int with suffix of k, m or g, meaning
                                 kilobits, megabits and gigabits per second ($BITRATE)
  -t, --threads=1                Number of threads to use, each with a separate connection and consuming specified
                                 bitrate ($NUM_THREADS)
      --buffer-min=1             Keep buffering and NOT start playing until reached ($BUFFER_MIN)
      --buffer-max=10            Stop buffering when reached ($BUFFER_MAX)
      --buffer-topped-delay=1    When buffer is full, how long to wait before trying beginning to refill it again
                                 ($BUFFER_TOPPED_DELAY)
```

There is bundled test server which provides random bytes (optionally at given bitrate):

```
Usage: dst server <port> [flags]

Run server which outputs random bytes to any connecting client

Arguments:
  <port>    Port to listen on ($PORT)

Flags:
  -h, --help                Show context-sensitive help.

  -b, --bitrate=BITRATE     Maximum bitrate for response, if desired. Must have suffix of k, m or g. By default
                            bitrate is not artificially limited and depends only on your system CSPRNG and networking
                            speed ($BITRATE)
      --random-bytes=INT    If set, only this number of random bytes will be generated, and then just cycled
                            to produce output. Can be used to remove throughput dependency on CSPRNG generator
                            performance ($RANDOM_BYTES)
```

## Docker image

See https://github.com/users/CthulhuDen/packages/container/package/dst.
