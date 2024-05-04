```
Usage: dst tester --bitrate=BITRATE <url> [flags]

Emulate video streaming at given bitrate to stress test you internet connection to given URL

Arguments:
  <url>    URL to connect to

Flags:
  -h, --help                           Show context-sensitive help.

  -b, --bitrate=BITRATE                Target video emulated bitrate. Must be int with suffix of kb or mb, meaning kilobits and megabits per second
      --buffer-min=SECONDS             Keep buffering and NOT start playing until reached
      --buffer-max=SECONDS             Stop buffering when reached
      --buffer-topped-delay=SECONDS    When buffer is full, how long to wait before trying beginning to refill it again
```

There is bundled test server which provides random bytes (optionally at given bitrate):
```
Usage: dst server <port> [flags]

Run server which outputs random bytes to any connecting client

Arguments:
  <port>    Port to listen on

Flags:
  -h, --help               Show context-sensitive help.

  -b, --bitrate=BITRATE    Maximum bitrate for response, if desired. Must have suffix of kb or mb
```
