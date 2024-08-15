package realip

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Popular Headers
const (
	HeaderXForwardedFor = "x-forwarded-for"
	HeaderXRealIP       = "x-real-ip"
	// we can also specify True-Client-IP, CF-Connecting-IP and so on to c.RealIPHeader
	headerForwarded = "forwarded"
)

// Handler is the main object that acts as a middleware much like
// nginx's `ngx_http_realip_module`.
//
// Handlers must be constructed using the `Builder`. Using the zero
// value will result in undefined behavior.
type Handler struct {
	trusted   []*net.IPNet
	srcHeader string // default: X-Real-IP. where to get the real IP from
	dstHeader string // default: X-Real-IP. where to set the replacement IP to
	recursive bool
	next      http.Handler
	muCache   *sync.RWMutex
	cache     map[string]struct{}
}

// Builder is used to construct a realip.Handler object.
type Builder struct {
	h   *Handler
	err error
}

// New creates a new realip.Builder, which is used to
// construct a realip.Handler.
//
// The default configuration trusts all IP ranges, and uses the
// X-Real-IP header as the request header field whose value will
// be used to replace the client address.
func New() *Builder {
	b := &Builder{}
	b.Reset()
	return b
}

func (b *Builder) Build() (*Handler, error) {
	h, err := b.h, b.err
	b.Reset()
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (b *Builder) Reset() *Builder {
	b.err = nil
	b.h = &Handler{
		srcHeader: HeaderXRealIP,
		dstHeader: HeaderXRealIP,
		cache:     make(map[string]struct{}),
		muCache:   &sync.RWMutex{},
	}
	return b
}

// SourceHeader sets the header field to read the real IP from.
// Because this module does not supported the `Forwarded` header yet,
// specifying `Forwarded` will result in the builder failing to build
// a realip.Handler
//
// This is equivalent to `real_ip_header` directive in
// ngx_http_realip_module.
func (b *Builder) SourceHeader(name string) *Builder {
	if b.err != nil {
		return b
	}
	lowered := strings.ToLower(strings.TrimSpace(name))
	if lowered == headerForwarded {
		b.err = errors.New("realip.Builder: `Forwarded` header is not supported")
		return b
	}

	b.h.srcHeader = lowered
	return b
}

// DestinationHeader sets the header field to set the real IP to.
func (b *Builder) DestinationHeader(header string) *Builder {
	if b.err != nil {
		return b
	}
	b.h.dstHeader = strings.ToLower(header)
	return b
}

// TrustedIP sets the list of IP ranges that are trusted to
// provide the real IP address of the client.
//
// This is equivalent to `set_real_ip_from` directive in
// ngx_http_realip_module.
func (b *Builder) TrustedIP(trusted ...*net.IPNet) *Builder {
	if b.err != nil {
		return b
	}
	b.h.trusted = append(b.h.trusted, trusted...)
	return b
}

// Recursive enables the recursive mode, which will look for
// the most recent non-trusted IP address in the header field.
//
// This is equivalent to `real_ip_recursive` directive in
// ngx_http_realip_module.
func (b *Builder) Recursive(v bool) *Builder {
	if b.err != nil {
		return b
	}
	b.h.recursive = v
	return b
}

// Wrap returns a http.Handler that wraps the given handler with the
// realip.Handler object, allowing you to use it as a middleware.
//
// For frameworks that require a http.HandleFunc instead of http.Handler,
// simply use this method and extract the `ServeHTTP` method from the
// return value, and pass it to the framework.
func (h *Handler) Wrap(next http.Handler) http.Handler {
	h.next = next
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawRemoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !h.trustedIP(net.ParseIP(rawRemoteIP), h.trusted) {
		if rawRemoteIP != "" {
			r.Header.Set(h.dstHeader, rawRemoteIP)
		}
		h.next.ServeHTTP(w, r)
		return
	}

	var realIP string
	switch h.srcHeader { // note: h.srcHeader is guaranteed to be lower cased
	case HeaderXForwardedFor:
		realIP = h.realIPFromXFF(r.Header.Get(HeaderXForwardedFor))
	default:
		realIP = r.Header.Get(h.srcHeader)
	}

	if realIP == "" {
		realIP = rawRemoteIP
	}
	if realIP != "" {
		r.Header.Set(h.dstHeader, realIP)
	}
	h.next.ServeHTTP(w, r)
}

func (h *Handler) trustedIP(ip net.IP, trusted []*net.IPNet) bool {
	if len(trusted) == 0 {
		// trust everybody
		return true
	}

	ipstr := ip.String()
	h.muCache.RLock()
	_, cached := h.cache[ipstr]
	h.muCache.RUnlock()
	if cached {
		return true
	}

	for _, fromIP := range trusted {
		if fromIP.Contains(ip) {
			h.muCache.Lock()
			h.cache[ipstr] = struct{}{}
			h.muCache.Unlock()
			return true
		}
	}
	return false
}

func (h *Handler) realIPFromXFF(xff string) string {
	ips := strings.Split(xff, ",")
	if len(ips) == 0 {
		return ""
	}

	if !h.recursive {
		return strings.TrimSpace(ips[len(ips)-1])
	}

	for i := len(ips) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(ips[i])
		if !h.trustedIP(net.ParseIP(ipStr), h.trusted) {
			return ipStr
		}
	}
	return strings.TrimSpace(ips[0])
}
