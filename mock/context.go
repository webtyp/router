package mock

import (
	"bytes"
	"sync"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Context buffers the response and lets tests set the request fields.
// Unlike the base router.Context contract (single-goroutine ownership),
// this mock IS safe for concurrent use: its role is to let the test
// goroutine observe (ResponseBody) while a handler goroutine writes.
type Context struct {
	InMethod string
	InPath   string
	InBody   []byte

	Status      int
	mu          sync.RWMutex
	response    bytes.Buffer
	headers     map[string]string
	cookies     map[string]router.Cookie
	values      map[string]string
	paramNames  []string
	paramValues []string
	userID      string
}

// ResponseBody returns a copy of the buffered response body.
func (c *Context) ResponseBody() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Returns a copy to avoid race conditions with subsequent writes.
	src := c.response.Bytes()
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func (c *Context) Method() string {
	if c.InMethod != "" {
		return c.InMethod
	}
	return "GET"
}

func (c *Context) Path() string {
	if c.InPath != "" {
		return c.InPath
	}
	return "/"
}

func (c *Context) Body() []byte {
	return c.InBody
}

func (c *Context) GetHeader(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.headers == nil {
		return ""
	}
	return c.headers[key]
}

func (c *Context) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

func (c *Context) WriteStatus(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = code
}

func (c *Context) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.response.Write(b)
}

func (c *Context) SetValue(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
}

func (c *Context) Value(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.values == nil {
		return ""
	}
	return c.values[key]
}

func (c *Context) SetParams(names, values []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paramNames = names
	c.paramValues = values
}

func (c *Context) Param(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i, n := range c.paramNames {
		if n == name && i < len(c.paramValues) {
			return c.paramValues[i]
		}
	}
	return ""
}

func (c *Context) SetCookie(cookie router.Cookie) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cookies == nil {
		c.cookies = make(map[string]router.Cookie)
	}
	c.cookies[cookie.Name] = cookie
}

func (c *Context) Cookie(name string) (router.Cookie, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cookies == nil {
		return router.Cookie{}, false
	}
	cookie, ok := c.cookies[name]
	return cookie, ok
}

func (c *Context) SetUserID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userID = id
}

func (c *Context) UserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

// Decode reads the request body as JSON into a typed destination. The mock backs Decode
// with a real codec (webtyp/json) rather than a fake, so a test proves the same
// round-trip a deployed transport performs.
func (c *Context) Decode(into model.Decodable) error {
	return json.Decode(c.Body(), into)
}

// Encode writes v as the JSON response body, through the same real codec as Decode.
func (c *Context) Encode(v model.Encodable) error {
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		return err
	}
	_, err := c.Write(out)
	return err
}

var _ router.Context = (*Context)(nil)
