package scanner

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestStripHTMLTags(t *testing.T) {
	input := `<html><head><title>Test Title</title><style>body { background: #fff; }</style><script>console.log("hello");</script></head><body><h1>Hello World</h1><p>Test paragraph.</p></body></html>`
	expected := "Test Title Hello World Test paragraph."
	got := stripHTMLTags(input)
	if got != expected {
		t.Errorf("stripHTMLTags() = %q, want %q", got, expected)
	}
}

func TestGrabHTTPInfoAndBanner(t *testing.T) {
	// TCP listener for Banner test
	bannerText := "220 Welcome to FTP service\r\n"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Write([]byte(bannerText))
			conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	gotBanner := grabBanner("127.0.0.1", port, 1*time.Second)
	expectedBanner := "220 Welcome to FTP service"
	if gotBanner != expectedBanner {
		t.Errorf("grabBanner() = %q, want %q", gotBanner, expectedBanner)
	}

	// HTTP test
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "TestServer/1.0")
		w.Write([]byte(`<html><head><title>AP Setup</title></head><body>Welcome to the AP configuration portal.</body></html>`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	_, httpPortStr, _ := net.SplitHostPort(u.Host)
	httpPort, _ := strconv.Atoi(httpPortStr)

	gotHTTP := grabHTTPInfo("127.0.0.1", httpPort, 2*time.Second)
	if !strings.Contains(gotHTTP, "Title: AP Setup") || !strings.Contains(gotHTTP, "Server: TestServer/1.0") || !strings.Contains(gotHTTP, "Text: AP Setup Welcome to the AP configuration portal.") {
		t.Errorf("grabHTTPInfo() returned unexpected result: %q", gotHTTP)
	}
}

