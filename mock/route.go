package mock

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

// Route implements router.Route for the mock, recording permission annotations.
type Route struct {
	info router.RouteInfo
}

func (r *Route) Requires(resource model.Resource, action model.Action) router.Route {
	r.info.Access = model.AccessGuarded
	r.info.Resource = resource
	r.info.Action = action
	return r
}

func (r *Route) Authenticated() router.Route {
	r.info.Access = model.AccessAuthenticated
	return r
}

func (r *Route) Public() router.Route {
	r.info.Access = model.AccessPublic
	return r
}

func (r *Route) Accepts(args model.Fielder) router.Route {
	r.info.Args = args
	return r
}

var _ router.Route = (*Route)(nil)
