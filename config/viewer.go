package config

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/tarunnahak/godevtool/internal/color"
)

// Entry represents a single configuration field.
type Entry struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type"`
	Source   string `json:"source,omitempty"` // e.g. "env:DATABASE_URL", "flag:port"
	Redacted bool   `json:"redacted"`
}

// ConfigSnapshot represents a named configuration section.
type ConfigSnapshot struct {
	Name    string  `json:"name"`
	Entries []Entry `json:"entries"`
}

// Viewer introspects and displays configuration structs.
type Viewer struct {
	mu      sync.RWMutex
	configs map[string]configRef
}

type configRef struct {
	name   string
	value  any
	source map[string]string // field name -> source description
}

// New creates a Viewer.
func New() *Viewer {
	return &Viewer{
		configs: make(map[string]configRef),
	}
}

// Register adds a named config struct for display.
// Sensitive fields tagged `devtool:"redact"` will have their values masked.
// Optionally provide sources as fieldName->source pairs.
func (v *Viewer) Register(name string, cfg any, sources ...map[string]string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	ref := configRef{name: name, value: cfg}
	if len(sources) > 0 {
		ref.source = sources[0]
	}
	v.configs[name] = ref
}

// Unregister removes a named config.
func (v *Viewer) Unregister(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.configs, name)
}

// Snapshot returns all registered configs as snapshots.
func (v *Viewer) Snapshot() []ConfigSnapshot {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var result []ConfigSnapshot
	for _, ref := range v.configs {
		snap := ConfigSnapshot{Name: ref.name}
		snap.Entries = inspectConfig(ref.value, ref.source)
		result = append(result, snap)
	}
	return result
}

// Get returns the snapshot for a specific config name.
func (v *Viewer) Get(name string) (ConfigSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	ref, ok := v.configs[name]
	if !ok {
		return ConfigSnapshot{}, false
	}
	snap := ConfigSnapshot{Name: ref.name}
	snap.Entries = inspectConfig(ref.value, ref.source)
	return snap, true
}

// Names returns all registered config names.
func (v *Viewer) Names() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var names []string
	for name := range v.configs {
		names = append(names, name)
	}
	return names
}

// FormatSnapshot returns a human-readable string of a config snapshot.
func FormatSnapshot(snap ConfigSnapshot, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap(snap.Name, colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	maxKey := 0
	for _, e := range snap.Entries {
		if len(e.Key) > maxKey {
			maxKey = len(e.Key)
		}
	}

	for _, e := range snap.Entries {
		b.WriteString("  ")
		b.WriteString(color.Wrap(fmt.Sprintf("%-*s", maxKey, e.Key), colorize, color.Blue))
		b.WriteString("  ")

		if e.Redacted {
			b.WriteString(color.Wrap("********", colorize, color.Red))
		} else {
			b.WriteString(e.Value)
		}

		b.WriteString(color.Wrap("  ("+e.Type+")", colorize, color.Gray))

		if e.Source != "" {
			b.WriteString(color.Wrap("  ["+e.Source+"]", colorize, color.Gray))
		}

		b.WriteByte('\n')
	}

	return b.String()
}

func inspectConfig(v any, sources map[string]string) []Entry {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return []Entry{{
			Key:   "(value)",
			Value: fmt.Sprintf("%v", v),
			Type:  val.Type().String(),
		}}
	}

	t := val.Type()
	var entries []Entry

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fv := val.Field(i)
		entry := Entry{
			Key:  field.Name,
			Type: field.Type.String(),
		}

		// check for redact tag
		tag := field.Tag.Get("devtool")
		if tag == "redact" {
			entry.Redacted = true
			entry.Value = "********"
		} else {
			entry.Value = formatConfigValue(fv)
		}

		// check for source
		if sources != nil {
			if src, ok := sources[field.Name]; ok {
				entry.Source = src
			}
		}

		// also check json/env/yaml tags as source hints
		if entry.Source == "" {
			if envTag := field.Tag.Get("env"); envTag != "" {
				entry.Source = "env:" + envTag
			} else if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" && parts[0] != "-" {
					entry.Source = "json:" + parts[0]
				}
			}
		}

		entries = append(entries, entry)

		// recurse into nested structs
		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(struct{}{}) {
			// skip time.Time and similar stdlib structs
			if fv.Type().PkgPath() != "time" {
				nested := inspectConfig(fv.Interface(), sources)
				for _, ne := range nested {
					ne.Key = field.Name + "." + ne.Key
					entries = append(entries, ne)
				}
			}
		}
	}

	return entries
}

func formatConfigValue(v reflect.Value) string {
	if !v.IsValid() {
		return "<nil>"
	}

	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if s == "" {
			return `""`
		}
		return s
	case reflect.Slice, reflect.Array:
		if v.IsNil() || v.Len() == 0 {
			return "[]"
		}
		var parts []string
		for i := 0; i < v.Len() && i < 10; i++ {
			parts = append(parts, fmt.Sprintf("%v", v.Index(i).Interface()))
		}
		if v.Len() > 10 {
			parts = append(parts, fmt.Sprintf("... (%d more)", v.Len()-10))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case reflect.Map:
		if v.IsNil() || v.Len() == 0 {
			return "{}"
		}
		var parts []string
		count := 0
		for _, key := range v.MapKeys() {
			if count >= 5 {
				parts = append(parts, fmt.Sprintf("... (%d more)", v.Len()-5))
				break
			}
			parts = append(parts, fmt.Sprintf("%v:%v", key.Interface(), v.MapIndex(key).Interface()))
			count++
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case reflect.Ptr:
		if v.IsNil() {
			return "<nil>"
		}
		return formatConfigValue(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}
