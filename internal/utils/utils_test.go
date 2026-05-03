package utils

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500B"},
		{1024, "1.00Ki"},
		{1536, "1.50Ki"},
		{1024 * 1024, "1.00Mi"},
		{1024 * 1024 * 1024, "1.00Gi"},
		{0, "0B"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.bytes); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestMin(t *testing.T) {
	if Min(3, 7) != 3 {
		t.Errorf("Min(3,7) want 3")
	}
	if Min(9, 2) != 2 {
		t.Errorf("Min(9,2) want 2")
	}
}

func TestInt32Ptr(t *testing.T) {
	v := int32(42)
	p := Int32Ptr(v)
	if p == nil || *p != v {
		t.Fatalf("Int32Ptr: got %v", p)
	}
}
