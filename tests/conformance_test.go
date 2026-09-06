package router_test

import (
	"testing"

	"webtyp.com/router"
	"webtyp.com/router/conformance"
	"webtyp.com/router/mock"
)

// mockUserHeader is how this test's authentication seam learns who is calling. A real
// implementation reads a cookie or a bearer token; the shape is the same and that is the
// point — identity arrives with the request and Authn turns it into a UserID.
const mockUserHeader = "X-Mock-User"

// TestMockConformance holds the mock to the same contract the deployed implementations must
// meet. It is not ceremony: consumers across the ecosystem write their tests against this
// mock, so a mock that cannot reject would let every one of them pass while their access
// control is broken in production.
func TestMockConformance(t *testing.T) {
	conformance.Run(t, conformance.Factory{
		New: func(t *testing.T, s conformance.Setup) (router.Router, conformance.ServeFunc) {
			r := &mock.Router{}
			r.Configure(mock.Config{
				Authn: func(next router.HandlerFunc) router.HandlerFunc {
					return func(ctx router.Context) {
						if id := ctx.GetHeader(mockUserHeader); id != "" {
							ctx.SetUserID(id)
						}
						next(ctx)
					}
				},
				Authorize: s.Authorize,
			})

			serve := func(method, path string, body []byte, userID string) conformance.Response {
				ctx := &mock.Context{InMethod: method, InPath: path, InBody: body}
				if userID != "" {
					ctx.SetHeader(mockUserHeader, userID)
				}
				r.Invoke(method, path, ctx)
				return conformance.Response{Status: ctx.Status, Body: ctx.ResponseBody()}
			}
			return r, serve
		},

		Verify: func(r router.Router) error { return r.(*mock.Router).Verify() },

		// The mock registers Op under the synthetic method "OP" + path "/"+name — its own
		// implementation detail (see mock.Router.Op's doc). ServeOp is exactly the seam that
		// lets that detail stay internal: conformance never needs to know it.
		ServeOp: func(r router.Router, name string, body []byte, userID string) conformance.Response {
			ctx := &mock.Context{InMethod: "OP", InPath: "/" + name, InBody: body}
			if userID != "" {
				ctx.SetHeader(mockUserHeader, userID)
			}
			r.(*mock.Router).Invoke("OP", "/"+name, ctx)
			return conformance.Response{Status: ctx.Status, Body: ctx.ResponseBody()}
		},
	})
}
