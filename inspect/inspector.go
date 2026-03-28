package inspect

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/nahaktarun/godevtool/internal/color"
)

// Config controls inspection behavior.
type Config struct {
	MaxDepth    int
	Colorize    bool
	ShowPrivate bool
	ShowTags    bool
	Output      io.Writer
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxDepth:    10,
		Colorize:    true,
		ShowPrivate: true,
		ShowTags:    false,
		Output:      os.Stdout,
	}
}

// Sprint returns a pretty-printed string of v.
func Sprint(v any, cfg Config) string {
	var b strings.Builder
	inspect(&b, reflect.ValueOf(v), cfg, 0, make(map[uintptr]bool))
	return b.String()
}

// Fprint writes the pretty-printed inspection to w.
func Fprint(w io.Writer, v any, cfg Config) {
	fmt.Fprint(w, Sprint(v, cfg))
}

// Print writes the inspection to the configured output (default stdout).
func Print(v any, cfg Config) {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprint(out, Sprint(v, cfg))
}

func inspect(b *strings.Builder, v reflect.Value, cfg Config, depth int, visited map[uintptr]bool) {
	if depth > cfg.MaxDepth {
		b.WriteString(color.Wrap("...", cfg.Colorize, color.Gray))
		return
	}

	if !v.IsValid() {
		b.WriteString(color.Wrap("<nil>", cfg.Colorize, color.Gray))
		return
	}

	switch v.Kind() {
	case reflect.Ptr:
		inspectPtr(b, v, cfg, depth, visited)
	case reflect.Struct:
		inspectStruct(b, v, cfg, depth, visited)
	case reflect.Slice, reflect.Array:
		inspectSlice(b, v, cfg, depth, visited)
	case reflect.Map:
		inspectMap(b, v, cfg, depth, visited)
	case reflect.Interface:
		if v.IsNil() {
			b.WriteString(typeName(v, cfg))
			b.WriteString(color.Wrap(" <nil>", cfg.Colorize, color.Gray))
		} else {
			inspect(b, v.Elem(), cfg, depth, visited)
		}
	case reflect.Chan, reflect.Func:
		b.WriteString(typeName(v, cfg))
		if v.IsNil() {
			b.WriteString(color.Wrap(" <nil>", cfg.Colorize, color.Gray))
		} else {
			b.WriteString(fmt.Sprintf(" %v", v))
		}
	default:
		inspectScalar(b, v, cfg)
	}
}

func inspectPtr(b *strings.Builder, v reflect.Value, cfg Config, depth int, visited map[uintptr]bool) {
	if v.IsNil() {
		b.WriteString(color.Wrap("(*"+v.Type().Elem().String()+")", cfg.Colorize, color.Cyan))
		b.WriteString(color.Wrap(" <nil>", cfg.Colorize, color.Gray))
		return
	}

	ptr := v.Pointer()
	if visited[ptr] {
		b.WriteString(color.Wrap("<circular ref>", cfg.Colorize, color.Red))
		return
	}
	visited[ptr] = true

	b.WriteString(color.Wrap("(*"+v.Type().Elem().String()+")", cfg.Colorize, color.Cyan))
	b.WriteByte(' ')
	inspect(b, v.Elem(), cfg, depth, visited)
}

func inspectStruct(b *strings.Builder, v reflect.Value, cfg Config, depth int, visited map[uintptr]bool) {
	t := v.Type()
	b.WriteString(color.Wrap("("+t.String()+")", cfg.Colorize, color.Cyan))
	b.WriteString(" {\n")

	indent := strings.Repeat("  ", depth+1)
	maxKeyLen := 0
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !cfg.ShowPrivate && !f.IsExported() {
			continue
		}
		if len(f.Name) > maxKeyLen {
			maxKeyLen = len(f.Name)
		}
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !cfg.ShowPrivate && !f.IsExported() {
			continue
		}

		b.WriteString(indent)
		name := fmt.Sprintf("%-*s", maxKeyLen, f.Name)
		b.WriteString(color.Wrap(name, cfg.Colorize, color.Blue))
		b.WriteString(": ")

		fv := v.Field(i)
		if f.IsExported() {
			inspect(b, fv, cfg, depth+1, visited)
		} else {
			// unexported: show type and zero-value indicator
			b.WriteString(color.Wrap("("+f.Type.String()+")", cfg.Colorize, color.Gray))
			b.WriteString(" <unexported>")
		}

		if cfg.ShowTags && f.Tag != "" {
			b.WriteString(color.Wrap(" `"+string(f.Tag)+"`", cfg.Colorize, color.Gray))
		}
		b.WriteByte('\n')
	}

	b.WriteString(strings.Repeat("  ", depth))
	b.WriteByte('}')
}

func inspectSlice(b *strings.Builder, v reflect.Value, cfg Config, depth int, visited map[uintptr]bool) {
	t := v.Type()
	if v.Kind() == reflect.Slice && v.IsNil() {
		b.WriteString(color.Wrap("("+t.String()+")", cfg.Colorize, color.Cyan))
		b.WriteString(color.Wrap(" <nil>", cfg.Colorize, color.Gray))
		return
	}

	length := v.Len()
	b.WriteString(color.Wrap(fmt.Sprintf("(%s)", t), cfg.Colorize, color.Cyan))
	b.WriteString(color.Wrap(fmt.Sprintf(" [%d items]", length), cfg.Colorize, color.Gray))

	if length == 0 {
		b.WriteString(" []")
		return
	}

	b.WriteString(" [\n")
	indent := strings.Repeat("  ", depth+1)

	maxItems := 50
	if length < maxItems {
		maxItems = length
	}

	for i := 0; i < maxItems; i++ {
		b.WriteString(indent)
		b.WriteString(color.Wrap(fmt.Sprintf("[%d] ", i), cfg.Colorize, color.Gray))
		inspect(b, v.Index(i), cfg, depth+1, visited)
		b.WriteByte('\n')
	}

	if length > maxItems {
		b.WriteString(indent)
		b.WriteString(color.Wrap(fmt.Sprintf("... (%d more)", length-maxItems), cfg.Colorize, color.Gray))
		b.WriteByte('\n')
	}

	b.WriteString(strings.Repeat("  ", depth))
	b.WriteByte(']')
}

func inspectMap(b *strings.Builder, v reflect.Value, cfg Config, depth int, visited map[uintptr]bool) {
	t := v.Type()
	if v.IsNil() {
		b.WriteString(color.Wrap("("+t.String()+")", cfg.Colorize, color.Cyan))
		b.WriteString(color.Wrap(" <nil>", cfg.Colorize, color.Gray))
		return
	}

	length := v.Len()
	b.WriteString(color.Wrap(fmt.Sprintf("(%s)", t), cfg.Colorize, color.Cyan))
	b.WriteString(color.Wrap(fmt.Sprintf(" [%d entries]", length), cfg.Colorize, color.Gray))

	if length == 0 {
		b.WriteString(" {}")
		return
	}

	b.WriteString(" {\n")
	indent := strings.Repeat("  ", depth+1)

	for _, key := range v.MapKeys() {
		b.WriteString(indent)
		b.WriteString(color.Wrap(fmt.Sprintf("%v", key.Interface()), cfg.Colorize, color.Blue))
		b.WriteString(": ")
		inspect(b, v.MapIndex(key), cfg, depth+1, visited)
		b.WriteByte('\n')
	}

	b.WriteString(strings.Repeat("  ", depth))
	b.WriteByte('}')
}

func inspectScalar(b *strings.Builder, v reflect.Value, cfg Config) {
	t := v.Type()
	b.WriteString(color.Wrap("("+t.String()+")", cfg.Colorize, color.Cyan))
	b.WriteByte(' ')

	switch v.Kind() {
	case reflect.String:
		s := v.String()
		b.WriteString(color.Wrap(fmt.Sprintf("%q", s), cfg.Colorize, color.Green))
	case reflect.Bool:
		b.WriteString(color.Wrap(fmt.Sprintf("%v", v.Bool()), cfg.Colorize, color.Yellow))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(color.Wrap(fmt.Sprintf("%d", v.Int()), cfg.Colorize, color.White))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b.WriteString(color.Wrap(fmt.Sprintf("%d", v.Uint()), cfg.Colorize, color.White))
	case reflect.Float32, reflect.Float64:
		b.WriteString(color.Wrap(fmt.Sprintf("%g", v.Float()), cfg.Colorize, color.White))
	case reflect.Complex64, reflect.Complex128:
		b.WriteString(fmt.Sprintf("%v", v.Complex()))
	default:
		b.WriteString(fmt.Sprintf("%v", v.Interface()))
	}
}

func typeName(v reflect.Value, cfg Config) string {
	return color.Wrap("("+v.Type().String()+")", cfg.Colorize, color.Cyan)
}
