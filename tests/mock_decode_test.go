package router_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/router/routertest"
)

// The mock Context decodes the arguments the way every deployed transport does (router.DecodeArgs): a GET carries them in its query,
// every other method in its JSON body. A handler tested on the mock therefore sees what it will see behind httpd or the edge.

func TestMockContext_PathAndRawQuery(t *testing.T) {
	ctx := &mock.Context{InPath: "/p?a=1"}
	if got := ctx.Path(); got != "/p" {
		t.Errorf("Path() = %q, want %q", got, "/p")
	}
	if got := ctx.RawQuery(); got != "a=1" {
		t.Errorf("RawQuery() = %q, want %q", got, "a=1")
	}

	ctx2 := &mock.Context{InPath: ""}
	if got := ctx2.Path(); got != "/" {
		t.Errorf("Path() of empty = %q, want %q", got, "/")
	}
	if got := ctx2.RawQuery(); got != "" {
		t.Errorf("RawQuery() of empty = %q, want empty", got)
	}
}

func TestMockContext_GetDecodesFromTheQuery(t *testing.T) {
	ctx := &mock.Context{InMethod: "GET", InPath: "/items?code=abc&qty=3"}
	var got targetArgs
	if err := ctx.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Code != "abc" || got.Qty != 3 {
		t.Fatalf("got %+v, want code abc and qty 3", got)
	}
}

func TestMockContext_GetIgnoresABody(t *testing.T) {
	ctx := &mock.Context{InMethod: "GET", InPath: "/items?code=abc", InBody: []byte(`{"code":"other"}`)}
	var got targetArgs
	if err := ctx.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Code != "abc" {
		t.Fatalf("a GET must read its query, never a body: got %q", got.Code)
	}
}

func TestMockContext_PostDecodesTheJSONBody(t *testing.T) {
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		ctx := &mock.Context{InMethod: method, InPath: "/items", InBody: []byte(`{"code":"z","qty":2}`)}
		var got targetArgs
		if err := ctx.Decode(&got); err != nil {
			t.Fatalf("%s: Decode: %v", method, err)
		}
		if got.Code != "z" || got.Qty != 2 {
			t.Fatalf("%s: got %+v, want code z and qty 2", method, got)
		}
	}
}

func TestServe_ServesAMockRouterOverHTTP(t *testing.T) {
	r := &mock.Router{}
	r.Get("/items", func(ctx router.Context) {
		var in targetArgs
		if err := ctx.Decode(&in); err != nil {
			ctx.WriteStatus(400)
			return
		}
		ctx.SetHeader("X-Seen", in.Code)
		ctx.WriteStatus(200)
		ctx.Write([]byte("code=" + in.Code))
	}).Public()
	r.Post("/items", func(ctx router.Context) {
		var in targetArgs
		if err := ctx.Decode(&in); err != nil {
			ctx.WriteStatus(400)
			return
		}
		ctx.WriteStatus(201)
		ctx.Write([]byte("made=" + in.Code))
	}).Public()

	srv := routertest.Serve(t, r)

	resp, err := http.Get(srv.URL + "/items?code=abc")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "code=abc" {
		t.Fatalf("GET: %d %q, want 200 code=abc", resp.StatusCode, body)
	}
	if got := resp.Header.Get("X-Seen"); got != "abc" {
		t.Errorf("response header X-Seen = %q, want abc (a header the handler set must reach the client)", got)
	}

	resp, err = http.Post(srv.URL+"/items", "application/json", strings.NewReader(`{"code":"z"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || string(body) != "made=z" {
		t.Fatalf("POST: %d %q, want 201 made=z", resp.StatusCode, body)
	}

	resp, err = http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("an unmatched path must answer 404, got %d", resp.StatusCode)
	}
}

func TestHandler_CanBeWrappedByATest(t *testing.T) {
	r := &mock.Router{}
	r.Get("/ping", func(ctx router.Context) { ctx.WriteStatus(200); ctx.Write([]byte("pong")) }).Public()
	hits := 0
	inner := routertest.Handler(r)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
		inner.ServeHTTP(w, req)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "pong" || hits != 1 {
		t.Fatalf("body %q, hits %d; want pong and 1 (a wrapper must see the request and still reach the router)", body, hits)
	}
}
