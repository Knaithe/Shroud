package cli

type KeyEvent struct {
	Key  int
	Char rune
	Err  error
}

const (
	KeyNone = iota
	KeyBackspace
	KeyEnter
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyTab
	KeyCtrlC
	KeySpace
)

type Terminal interface {
	Init() error
	Close()
	PollEvent() KeyEvent
	Interrupt()
}
