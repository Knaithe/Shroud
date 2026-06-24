package cli

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type scriptTerminal struct {
	reader *bufio.Reader
}

func NewScriptTerminal() Terminal {
	return &scriptTerminal{reader: bufio.NewReader(os.Stdin)}
}

func (t *scriptTerminal) Init() error { return nil }
func (t *scriptTerminal) Close()      {}
func (t *scriptTerminal) Interrupt()   {}

func (t *scriptTerminal) ReadLine() (string, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (t *scriptTerminal) PollEvent() KeyEvent {
	ch, _, err := t.reader.ReadRune()
	if err != nil {
		if err == io.EOF {
			return KeyEvent{Key: KeyCtrlC}
		}
		return KeyEvent{Key: KeyCtrlC, Err: err}
	}
	switch ch {
	case '\n', '\r':
		return KeyEvent{Key: KeyEnter}
	case '\t':
		return KeyEvent{Key: KeyTab}
	case '\b', 0x7f:
		return KeyEvent{Key: KeyBackspace}
	default:
		return KeyEvent{Char: ch}
	}
}
