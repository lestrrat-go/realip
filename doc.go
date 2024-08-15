/*
Package realip provides clients with "real ip" detection mechanisms from http.Request
in Go's HTTP middleware layer, similar to Nginx's ngx_http_realip_module.

		_, ipnet, _ := net.ParseCIDR("192.168.0.0/16")
		h, _ := realip.New().
		    TrustedIP(ipnet).
			SourceHeader(realip.HeaderXForwardedFor).
			Recurseive(true).
			Build()
		handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintf(w, req.Header.Get(realip.HeaderXRealIP))
	    })
		l, _ := net.Listen("tcp", "127.0.0.1:8080")

		http.Serve(l, h.Wrap(handler))
*/
package realip
