package routertest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/router/mock"
)

// Serve starts a real HTTP server (net/http/httptest, closed when the test ends) in front of a mock router: every request becomes a
// mock.Context, goes through the router's whole pipeline (identity, access gate, middleware, handler) and its buffered response is written
// back. It lets a library that talks HTTP (a client, a REST mapping) be tested end to end with the standard library alone, with no
// transport library such as httpd as a dependency.
//
//	srv := routertest.Serve(t, mockRouter)
//	resp, err := http.Get(srv.URL + "/api/patient?id=7")
func Serve(t testing.TB, r *mock.Router) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(r))
	t.Cleanup(srv.Close)
	return srv
}

// Handler is the http.Handler behind Serve, for a test that wraps it (counts requests, injects a failure) or serves it on its own listener.
//
//	h := routertest.Handler(mockRouter)
//	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { count(req); h.ServeHTTP(w, req) }))
func Handler(r *mock.Router) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		ctx := &mock.Context{InMethod: req.Method, InPath: req.URL.RequestURI(), InBody: body}
		sent := make(map[string]string, len(req.Header))
		for key := range req.Header {
			value := req.Header.Get(key)
			ctx.SetHeader(key, value)
			sent[key] = value
		}
		r.Invoke(req.Method, req.URL.Path, ctx)
		for key, value := range ctx.Headers() {
			if before, wasSent := sent[key]; !wasSent || before != value {
				w.Header().Set(key, value)
			}
		}
		status := ctx.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		w.Write(ctx.ResponseBody())
	})
}
