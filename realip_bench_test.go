//go:build ignore

// To run benchmarks:
//   - uncomment the above go:build directive
//   - run go mod tidy to pull in github.com/natureglobal/realip
//   - run go test -bench .

package realip_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lestrrat-go/realip"
	nature "github.com/natureglobal/realip"
)

func benchHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func runBenchmark(b *testing.B, h http.Handler) {
	ts := httptest.NewServer(h)
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	req.Header.Add("X-Forwarded-For", "1.2.3.4, 1.1.1.1, 192.168.0.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.Client().Do(req)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	h, _ := realip.New().
		SourceHeader(realip.HeaderXForwardedFor).
		TrustedIP(
			mustParseCIDR("127.0.0.1/32"),
			mustParseCIDR("192.168.0.0/16"),
		).
		Build()

	runBenchmark(b, h.Wrap(http.HandlerFunc(benchHandler)))
}

func BenchmarkNatureMiddleware(b *testing.B) {
	m := nature.MustMiddleware(&nature.Config{
		RealIPFrom: []*net.IPNet{
			mustParseCIDR("127.0.0.1/32"),
			mustParseCIDR("192.168.0.0/16"),
		},
		RealIPHeader: nature.HeaderXForwardedFor,
	})
	runBenchmark(b, m(http.HandlerFunc(benchHandler)))
}
