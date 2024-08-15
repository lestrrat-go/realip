package realip_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lestrrat-go/realip"
)

func mustParseCIDR(addr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(addr)
	if err != nil {
		panic(err)
	}
	return ipnet
}

func TestMiddleware(t *testing.T) {
	testCases := []struct {
		name    string
		headers map[string]string
		expect  string
		create  func() (*realip.Handler, error)
		error   bool
	}{
		{
			name:   "header `Forwarded` is not supported",
			create: realip.New().SourceHeader("Forwarded").Build,
			error:  true,
		},
		{
			name:   "X-Real-IP: default",
			expect: "127.0.0.1",
			create: realip.New().Build,
		},
		{
			name: "X-Real-IP: no RealIPFrom",
			headers: map[string]string{
				"x-real-ip": "1.1.1.1",
			},
			expect: "1.1.1.1",
			create: realip.New().Build,
		},
		{
			name: "X-Real-IP: not a trusted ip from",
			headers: map[string]string{
				"x-real-ip": "1.1.1.1",
			},
			expect: "127.0.0.1",
			create: realip.New().TrustedIP(mustParseCIDR("192.168.0.0/16")).Build,
		},
		{
			name: "x-forwarded-for",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.0.1",
			},
			expect: "192.168.0.1",
			create: realip.New().SourceHeader(realip.HeaderXForwardedFor).Build,
		},
		{
			name: "x-forwarded-for: recent non-trusted one",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4, 1.1.1.1, 192.168.0.1",
			},
			expect: "1.1.1.1",
			create: realip.New().
				SourceHeader(realip.HeaderXForwardedFor).
				TrustedIP(
					mustParseCIDR("127.0.0.1/32"),
					mustParseCIDR("192.168.0.0/16"),
				).
				Recursive(true).
				Build,
		},
		{
			name: "x-forwarded-for: recent trusted one",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4, 1.1.1.1, 192.168.0.1",
			},
			expect: "192.168.0.1",
			create: realip.New().
				TrustedIP(
					mustParseCIDR("127.0.0.1/32"),
					mustParseCIDR("192.168.0.0/16"),
				).
				SourceHeader(realip.HeaderXForwardedFor).
				Build,
		},
		{
			name: "x-forwarded-for: remoteAddr is not a trusted address",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4, 1.1.1.1, 192.168.0.1",
			},
			expect: "127.0.0.1",
			create: realip.New().
				TrustedIP(
					mustParseCIDR("192.168.0.0/16"),
				).
				SourceHeader(realip.HeaderXForwardedFor).
				Build,
		},
		{
			name: "x-forwarded-for: all entries in xff is trusted ip",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.0.2, 192.168.0.1",
			},
			expect: "192.168.0.2",
			create: realip.New().
				TrustedIP(
					mustParseCIDR("127.0.0.1/32"),
					mustParseCIDR("192.168.0.0/16"),
				).
				SourceHeader(realip.HeaderXForwardedFor).
				Recursive(true).
				Build,
		},
		{
			name: "x-forwarded-for: no RealIPFrom config and true RealIPRecursive return left entry in xff",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.0.2, 192.168.0.1",
			},
			expect: "192.168.0.2",
			create: realip.New().
				SourceHeader(realip.HeaderXForwardedFor).
				Recursive(true).
				Build,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := tc.create()
			if tc.error {
				if err == nil {
					t.Fatal("building should fail, but received no error")
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			handler := h.Wrap(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				fmt.Fprint(w, req.Header.Get(realip.HeaderXRealIP))
			}))
			ts := httptest.NewServer(handler)
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
			if tc.headers != nil {
				for k, v := range tc.headers {
					req.Header.Add(k, v)
				}
			}
			r, _ := http.DefaultClient.Do(req)
			defer r.Body.Close()

			data, _ := io.ReadAll(r.Body)
			out := strings.TrimSpace(string(data))
			if out != tc.expect {
				t.Errorf("out: %s, expect: %s", out, tc.expect)
			}
		})
	}
}
