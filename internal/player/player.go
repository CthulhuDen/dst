package player

import (
	"io"
	"time"
)

type Emulator struct {
	b *Buffer
}

func NewEmulator(b *Buffer) *Emulator {
	return &Emulator{b: b}
}

func (e *Emulator) Run() error {
	for {
		err := e.b.GetNextFrame(24)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		time.Sleep(time.Second / 24)
	}
}
