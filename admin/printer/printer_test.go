package printer

import (
	"testing"
)

func TestInitPrinter_SetsNonNil(t *testing.T) {
	// Reset to nil to verify InitPrinter sets them.
	Warning = nil
	Success = nil
	Fail = nil

	InitPrinter()

	if Warning == nil {
		t.Error("Warning is nil after InitPrinter")
	}
	if Success == nil {
		t.Error("Success is nil after InitPrinter")
	}
	if Fail == nil {
		t.Error("Fail is nil after InitPrinter")
	}
}

func TestInitPrinter_Idempotent(t *testing.T) {
	InitPrinter()
	w1 := Warning
	s1 := Success
	f1 := Fail

	InitPrinter()

	// After second init, functions should still be non-nil.
	if Warning == nil || Success == nil || Fail == nil {
		t.Error("functions became nil after second InitPrinter")
	}

	// They are new function values each time, so identity may differ.
	// Just verify non-nil.
	_ = w1
	_ = s1
	_ = f1
}

func TestWarning_NoPanic(t *testing.T) {
	InitPrinter()
	// Calling Warning should not panic.
	Warning("[test] warning %s %d\r\n", "hello", 42)
}

func TestSuccess_NoPanic(t *testing.T) {
	InitPrinter()
	// Calling Success should not panic.
	Success("[test] success %s\r\n", "ok")
}

func TestFail_NoPanic(t *testing.T) {
	InitPrinter()
	// Calling Fail should not panic.
	Fail("[test] fail %d\r\n", 1)
}

func TestPrintFunctions_NoArgs(t *testing.T) {
	InitPrinter()
	// Calling with format-only (no variadic args) should not panic.
	Warning("plain warning message")
	Success("plain success message")
	Fail("plain fail message")
}

func TestPrintFunctions_MultipleArgs(t *testing.T) {
	InitPrinter()
	Warning("a=%s b=%d c=%v", "x", 1, true)
	Success("a=%s b=%d c=%v", "y", 2, false)
	Fail("a=%s b=%d c=%v", "z", 3, nil)
}
