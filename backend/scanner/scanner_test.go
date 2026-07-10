package scanner

import (
	"reflect"
	"testing"
)

func TestGenerateIPs(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    []string
		wantErr bool
	}{
		{
			name:   "Single IP",
			target: "192.168.1.1",
			want:   []string{"192.168.1.1"},
		},
		{
			name:   "CIDR Subnet",
			target: "192.168.1.0/30",
			want:   []string{"192.168.1.1", "192.168.1.2"},
		},
		{
			name:   "IP Range",
			target: "192.168.1.10-192.168.1.13",
			want:   []string{"192.168.1.10", "192.168.1.11", "192.168.1.12", "192.168.1.13"},
		},
		{
			name:   "Multiple comma-separated targets",
			target: "192.168.1.1, 192.168.1.10-192.168.1.12, 192.168.1.2",
			want:   []string{"192.168.1.1", "192.168.1.10", "192.168.1.11", "192.168.1.12", "192.168.1.2"},
		},
		{
			name:   "Duplicates in comma-separated list",
			target: "192.168.1.1, 192.168.1.1-192.168.1.2, 192.168.1.2",
			want:   []string{"192.168.1.1", "192.168.1.2"},
		},
		{
			name:    "Invalid target format",
			target:  "192.168.1.abc",
			wantErr: true,
		},
		{
			name:    "Invalid target in list",
			target:  "192.168.1.1, 192.168.1.abc",
			wantErr: true,
		},
		{
			name:    "Empty target list",
			target:  "  ,  ,  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPs(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateIPs() = %v, want %v", got, tt.want)
			}
		})
	}
}
