package store

import (
	"fmt"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// Timestamp is an instant in UTC. A v1 record carried a calendar date; both
// forms are read and only the instant is written back, so a reader comparing
// two records never has to know which generation wrote them.
type Timestamp struct{ time.Time }

const (
	timestampLayout = time.RFC3339
	dateLayout      = "2006-01-02"
)

func Now() Timestamp { return Timestamp{time.Now().UTC().Truncate(time.Second)} }

func ParseTimestamp(text string) (Timestamp, error) {
	if instant, err := time.Parse(timestampLayout, text); err == nil {
		return Timestamp{instant.UTC()}, nil
	}
	if day, err := time.Parse(dateLayout, text); err == nil {
		return Timestamp{day.UTC()}, nil
	}
	return Timestamp{}, fmt.Errorf("%q is neither an RFC3339 instant nor a %s date", text, dateLayout)
}

func (t Timestamp) MarshalYAML() (any, error) { return t.UTC().Format(timestampLayout), nil }

// UnmarshalYAML reads the node rather than a decoded value because an
// unquoted 2026-08-05 resolves to !!timestamp, which will not decode into a
// string at all. Reading Value sidesteps tag resolution entirely.
func (t *Timestamp) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("a timestamp is a single value, got %s", node.Tag)
	}
	parsed, err := ParseTimestamp(node.Value)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}
