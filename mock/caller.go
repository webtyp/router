package mock

import (
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Call records an invocation of a Caller operation.
type Call struct {
	Op   string
	Args model.Encodable
}

// Caller fakes the call-side contract, recording operations and allowing
// canned responses to be configured.
type Caller struct {
	Calls []Call

	// CannedResult is the wire response decoded into the Call's `into` target.
	// The mock backs the decode with the real codec (webtyp/json) so a test
	// exercises the same round-trip a deployed transport performs. A mock — like
	// any router implementation — is infrastructure, so naming a concrete codec
	// here is allowed; a domain module or a view never does.
	CannedResult []byte
	// CannedError is passed to the Call callback.
	CannedError error
}

func (c *Caller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	c.Calls = append(c.Calls, Call{Op: op, Args: args})
	err := c.CannedError
	if err == nil && into != nil && len(c.CannedResult) > 0 {
		err = json.Decode(c.CannedResult, into)
	}
	if done != nil {
		done(err)
	}
}

func (c *Caller) Dispatch(op string, args model.Encodable) {
	c.Calls = append(c.Calls, Call{Op: op, Args: args})
}

var _ router.Caller = (*Caller)(nil)
