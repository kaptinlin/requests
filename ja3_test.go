package requests

import (
	"crypto/tls"
	"reflect"
	"testing"
)

func TestParseJA3String(t *testing.T) {
	tests := []struct {
		name    string
		ja3     string
		wantErr bool
		want    *JA3Spec
	}{
		{
			name: "Chrome 120",
			ja3:  Chrome120JA3,
			want: &JA3Spec{
				Version: 771, // TLS 1.3
				CipherSuites: []uint16{
					4865, 4866, 4867, 49195, 49199, 49196, 49200,
					52393, 52392, 49171, 49172, 156, 157, 47, 53,
				},
				Extensions: []uint16{
					0, 23, 65281, 10, 11, 35, 16, 5, 13, 18,
					51, 45, 43, 27, 17513,
				},
				EllipticCurves:      []tls.CurveID{29, 23, 24},
				EllipticCurvePoints: []uint8{0},
			},
		},
		{
			name: "Firefox 120",
			ja3:  Firefox120JA3,
			want: &JA3Spec{
				Version: 771, // TLS 1.3
				CipherSuites: []uint16{
					4865, 4867, 4866, 49195, 49199, 49196, 49200,
					52393, 52392, 49171, 49172, 156, 157, 47, 53,
				},
				Extensions: []uint16{
					0, 23, 65281, 10, 11, 35, 16, 5, 13, 18,
					51, 45, 43, 27, 17513, 41,
				},
				EllipticCurves:      []tls.CurveID{29, 23, 24, 25},
				EllipticCurvePoints: []uint8{0},
			},
		},
		{
			name:    "Invalid Format",
			ja3:     "invalid,format",
			wantErr: true,
		},
		{
			name:    "Invalid Version",
			ja3:     "abc,4865-4866,0-23,29-23,0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJA3String(tt.ja3)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJA3String() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseJA3String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTLSConfigFromJA3(t *testing.T) {
	tests := []struct {
		name    string
		ja3     string
		wantErr bool
		check   func(*tls.Config) error
	}{
		{
			name: "Chrome 120",
			ja3:  Chrome120JA3,
			check: func(cfg *tls.Config) error {
				if cfg.MinVersion != 771 {
					t.Errorf("Expected MinVersion 771, got %d", cfg.MinVersion)
				}
				if !reflect.DeepEqual(cfg.NextProtos, []string{"h2", "http/1.1"}) {
					t.Errorf("Expected NextProtos [h2 http/1.1], got %v", cfg.NextProtos)
				}
				return nil
			},
		},
		{
			name:    "Invalid JA3",
			ja3:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewTLSConfigFromJA3(tt.ja3)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTLSConfigFromJA3() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && err == nil {
				if err := tt.check(cfg); err != nil {
					t.Errorf("Config check failed: %v", err)
				}
			}
		})
	}
}
