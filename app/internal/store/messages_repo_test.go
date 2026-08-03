package store

import (
	"fmt"
	"testing"
	"time"
)

func TestMessageScannersUseMatchingColumnCounts(t *testing.T) {
	if _, err := scanStoredMessage(countingScanner{want: 15}); err != nil {
		t.Fatalf("stored message scanner: %v", err)
	}
	if _, err := scanReadAPIMessage(countingScanner{want: 10}); err != nil {
		t.Fatalf("read API message scanner: %v", err)
	}
}

type countingScanner struct {
	want int
}

func (s countingScanner) Scan(dest ...any) error {
	if len(dest) != s.want {
		return fmt.Errorf("got %d scan destinations, want %d", len(dest), s.want)
	}
	for _, value := range dest {
		switch target := value.(type) {
		case *int:
			*target = 1
		case *int64:
			*target = 1
		case *string:
			*target = "value"
		case *bool:
			*target = false
		case *time.Time:
			*target = time.Unix(1, 0).UTC()
		}
	}
	return nil
}
