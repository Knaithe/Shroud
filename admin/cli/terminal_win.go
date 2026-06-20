//go:build windows

package cli

import (
	"os"
	"os/signal"

	"github.com/eiannone/keyboard"
)

type kbTerminal struct {
	events <-chan keyboard.KeyEvent
	sigC   chan os.Signal
	done   chan struct{}
}

func NewTerminal() Terminal { return &kbTerminal{} }

func (t *kbTerminal) Init() error {
	events, err := keyboard.GetKeys(10)
	if err != nil {
		return err
	}
	t.events = events
	t.sigC = make(chan os.Signal, 1)
	t.done = make(chan struct{})
	signal.Notify(t.sigC, os.Interrupt)
	return nil
}

func (t *kbTerminal) Close() { keyboard.Close() }

func (t *kbTerminal) Interrupt() {
	select {
	case t.done <- struct{}{}:
	default:
	}
}

func (t *kbTerminal) PollEvent() KeyEvent {
	select {
	case ev := <-t.events:
		return KeyEvent{Key: mapKBKey(ev.Key), Char: ev.Rune, Err: ev.Err}
	case <-t.sigC:
		return KeyEvent{Key: KeyCtrlC}
	case <-t.done:
		return KeyEvent{Key: KeyNone}
	}
}

func mapKBKey(k keyboard.Key) int {
	switch k {
	case keyboard.KeyBackspace, keyboard.KeyBackspace2:
		return KeyBackspace
	case keyboard.KeyEnter:
		return KeyEnter
	case keyboard.KeyArrowUp:
		return KeyArrowUp
	case keyboard.KeyArrowDown:
		return KeyArrowDown
	case keyboard.KeyArrowLeft:
		return KeyArrowLeft
	case keyboard.KeyArrowRight:
		return KeyArrowRight
	case keyboard.KeyTab:
		return KeyTab
	case keyboard.KeyCtrlC:
		return KeyCtrlC
	case keyboard.KeySpace:
		return KeySpace
	default:
		return KeyNone
	}
}
