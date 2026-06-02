package hardware

import (
	"bytes"
	"testing"
)

func TestParseMAC(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:    "Valid Colon",
			input:   "AA:BB:CC:DD:EE:FF",
			want:    []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
			wantErr: false,
		},
		{
			name:    "Valid Hyphen",
			input:   "AA-BB-CC-DD-EE-FF",
			want:    []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
			wantErr: false,
		},
		{
			name:    "Valid Raw",
			input:   "AABBCCDDEEFF",
			want:    []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
			wantErr: false,
		},
		{
			name:    "Invalid Chars",
			input:   "ZZ:BB:CC:DD:EE:FF",
			wantErr: true,
		},
		{
			name:    "Invalid Length Short",
			input:   "AA:BB:CC",
			wantErr: true,
		},
		{
			name:    "Invalid Length Long",
			input:   "AA:BB:CC:DD:EE:FF:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMAC(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMAC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytes.Equal(got, tt.want) {
				t.Errorf("ParseMAC() = %v, want %v", got, tt.want)
			}
		})
	}
}
