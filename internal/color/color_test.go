package color

import "testing"

func TestWrap(t *testing.T) {
	// with color enabled
	result := Wrap("hello", true, Red)
	if result != "\033[31mhello\033[0m" {
		t.Errorf("expected ANSI wrapped string, got %q", result)
	}

	// with color disabled
	result = Wrap("hello", false, Red)
	if result != "hello" {
		t.Errorf("expected plain string, got %q", result)
	}

	// multiple codes
	result = Wrap("hello", true, Bold, Red)
	if result != "\033[1;31mhello\033[0m" {
		t.Errorf("expected multi-code ANSI string, got %q", result)
	}

	// no codes
	result = Wrap("hello", true)
	if result != "hello" {
		t.Errorf("expected plain string with no codes, got %q", result)
	}
}
