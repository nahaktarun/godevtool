package inspect

import (
	"strings"
	"testing"
)

type Address struct {
	Street string
	City   string
}

type Person struct {
	Name    string
	Age     int
	Address Address
	Tags    []string
}

func TestInspectStruct(t *testing.T) {
	p := Person{
		Name: "Alice",
		Age:  30,
		Address: Address{
			Street: "123 Main St",
			City:   "Springfield",
		},
		Tags: []string{"admin", "user"},
	}

	cfg := Config{MaxDepth: 10, Colorize: false, ShowPrivate: true}
	result := Sprint(p, cfg)

	checks := []string{
		"Person",
		"Name",
		`"Alice"`,
		"Age",
		"30",
		"Address",
		"Street",
		`"123 Main St"`,
		"Tags",
		"admin",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("expected %q in output:\n%s", check, result)
		}
	}
}

func TestInspectNilPointer(t *testing.T) {
	var p *Person
	cfg := Config{MaxDepth: 10, Colorize: false}
	result := Sprint(p, cfg)

	if !strings.Contains(result, "<nil>") {
		t.Errorf("expected <nil> in output: %s", result)
	}
}

func TestInspectMap(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	cfg := Config{MaxDepth: 10, Colorize: false}
	result := Sprint(m, cfg)

	if !strings.Contains(result, "map[string]int") {
		t.Errorf("expected type in output: %s", result)
	}
	if !strings.Contains(result, "2 entries") {
		t.Errorf("expected entry count: %s", result)
	}
}

func TestInspectSlice(t *testing.T) {
	s := []int{10, 20, 30}
	cfg := Config{MaxDepth: 10, Colorize: false}
	result := Sprint(s, cfg)

	if !strings.Contains(result, "3 items") {
		t.Errorf("expected item count: %s", result)
	}
	if !strings.Contains(result, "20") {
		t.Errorf("expected value 20: %s", result)
	}
}

func TestInspectNilSlice(t *testing.T) {
	var s []string
	cfg := Config{MaxDepth: 10, Colorize: false}
	result := Sprint(s, cfg)

	if !strings.Contains(result, "<nil>") {
		t.Errorf("expected <nil>: %s", result)
	}
}

func TestInspectMaxDepth(t *testing.T) {
	type Nested struct {
		Inner *Nested
	}
	n := &Nested{Inner: &Nested{Inner: &Nested{}}}
	cfg := Config{MaxDepth: 2, Colorize: false, ShowPrivate: true}
	result := Sprint(n, cfg)

	if !strings.Contains(result, "...") {
		t.Errorf("expected depth truncation: %s", result)
	}
}

func TestInspectScalars(t *testing.T) {
	cfg := Config{MaxDepth: 10, Colorize: false}

	if r := Sprint(42, cfg); !strings.Contains(r, "42") {
		t.Errorf("int: %s", r)
	}
	if r := Sprint("hello", cfg); !strings.Contains(r, `"hello"`) {
		t.Errorf("string: %s", r)
	}
	if r := Sprint(true, cfg); !strings.Contains(r, "true") {
		t.Errorf("bool: %s", r)
	}
	if r := Sprint(3.14, cfg); !strings.Contains(r, "3.14") {
		t.Errorf("float: %s", r)
	}
}
