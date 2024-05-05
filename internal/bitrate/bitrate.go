package bitrate

import (
	"bytes"
	"fmt"
	"strconv"
)

// Bitrate is bytes (!) per second
type Bitrate int

func (b *Bitrate) UnmarshalText(text []byte) error {
	var multiplier int
	text = bytes.ToLower(text)
	if bytes.HasSuffix(text, []byte("k")) {
		multiplier = 1024
	} else if bytes.HasSuffix(text, []byte("m")) {
		multiplier = 1024 * 1024
	} else if bytes.HasSuffix(text, []byte("g")) {
		multiplier = 1024 * 1024 * 1024
	} else {
		return fmt.Errorf("bitrate must end with k, m or g")
	}

	br, err := strconv.Atoi(string(text[:len(text)-1]))
	if err != nil {
		return fmt.Errorf("bitrate must be integer with suffix k, m or g")
	}

	if br <= 0 {
		return fmt.Errorf("bitrate must be positive")
	}

	*b = Bitrate(multiplier * br / 8)
	return nil
}
