package cli

import (
	"os"
	"os/signal"
	"syscall"
)

type daemonTerminal struct {
	sigCh chan os.Signal
}

func NewDaemonTerminal() Terminal {
	return &daemonTerminal{}
}

func (t *daemonTerminal) Init() error {
	t.sigCh = make(chan os.Signal, 1)
	signal.Notify(t.sigCh, syscall.SIGTERM, syscall.SIGINT)
	return nil
}

func (t *daemonTerminal) Close() {
	signal.Stop(t.sigCh)
}

func (t *daemonTerminal) PollEvent() KeyEvent {
	<-t.sigCh
	return KeyEvent{Key: KeyCtrlC}
}

func (t *daemonTerminal) Interrupt() {}
