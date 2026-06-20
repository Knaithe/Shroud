//go:build !windows

package cli

import "github.com/nsf/termbox-go"

type termboxTerminal struct{}

func NewTerminal() Terminal { return &termboxTerminal{} }

func (t *termboxTerminal) Init() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	termbox.SetCursor(0, 0)
	termbox.Flush()
	return nil
}

func (t *termboxTerminal) Close() { termbox.Close() }

func (t *termboxTerminal) Interrupt() { termbox.Interrupt() }

func (t *termboxTerminal) PollEvent() KeyEvent {
	ev := termbox.PollEvent()
	return KeyEvent{
		Key:  mapTermboxKey(ev.Key, ev.Type),
		Char: ev.Ch,
		Err:  ev.Err,
	}
}

func mapTermboxKey(k termbox.Key, evType termbox.EventType) int {
	if evType == termbox.EventInterrupt {
		return KeyNone
	}
	switch k {
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		return KeyBackspace
	case termbox.KeyEnter:
		return KeyEnter
	case termbox.KeyArrowUp:
		return KeyArrowUp
	case termbox.KeyArrowDown:
		return KeyArrowDown
	case termbox.KeyArrowLeft:
		return KeyArrowLeft
	case termbox.KeyArrowRight:
		return KeyArrowRight
	case termbox.KeyTab:
		return KeyTab
	case termbox.KeyCtrlC:
		return KeyCtrlC
	case termbox.KeySpace:
		return KeySpace
	default:
		return KeyNone
	}
}
