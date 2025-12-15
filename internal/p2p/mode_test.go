package p2p

import (
	"testing"
)

func TestParseNodeMode(t *testing.T) {
	tests := []struct {
		input    string
		expected NodeMode
		wantErr  bool
	}{
		{"proxy", NodeModeProxy, false},
		{"Proxy", NodeModeProxy, false},
		{"PROXY", NodeModeProxy, false},
		{"selective", NodeModeSelective, false},
		{"Selective", NodeModeSelective, false},
		{"full", NodeModeFull, false},
		{"Full", NodeModeFull, false},
		{"", NodeModeProxy, false}, // Empty defaults to proxy
		{"invalid", "", true},
		{"hybrid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := ParseNodeMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && mode != tt.expected {
				t.Errorf("ParseNodeMode(%q) = %v, want %v", tt.input, mode, tt.expected)
			}
		})
	}
}

func TestNodeModeString(t *testing.T) {
	tests := []struct {
		mode     NodeMode
		expected string
	}{
		{NodeModeProxy, "proxy"},
		{NodeModeSelective, "selective"},
		{NodeModeFull, "full"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("NodeMode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
