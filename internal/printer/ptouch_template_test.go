package printer

import "testing"

func TestEndpointFromDeviceURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"ipp://localhost:60000/ipp/print", "localhost:60000"},
		{"socket://192.168.50.40", "192.168.50.40:9100"},
		{"http://127.0.0.1/printer", "127.0.0.1:80"},
	}

	for _, tt := range tests {
		got, err := endpointFromDeviceURI(tt.uri)
		if err != nil {
			t.Fatalf("endpointFromDeviceURI(%q) returned error: %v", tt.uri, err)
		}
		if got != tt.want {
			t.Fatalf("endpointFromDeviceURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}
