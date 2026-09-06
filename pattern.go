package router

import (
	"webtyp.com/fmt"
)

const (
	ErrMsgWildcardUnsupported = "router: pattern %q: {name...} wildcards are not supported"
	ErrMsgEmptyParamName      = "router: pattern %q: empty parameter name"
	ErrMsgDuplicateParamName  = "router: pattern %q: duplicate parameter name %q"
	ErrMsgMixedSegment        = "router: pattern %q: segment %q mixes literal text and a parameter"
)

// ParamNames returns the parameter names pattern declares, left to right.
// nil when the pattern declares none.
func ParamNames(pattern string) []string {
	segs := splitSegments(pattern)
	var names []string
	for _, seg := range segs {
		if isParamSegment(seg) {
			names = append(names, seg[1:len(seg)-1])
		}
	}
	return names
}

// ValidatePattern reports why a pattern cannot be registered, or nil.
// A router MUST call it at registration and fail loudly — a bad pattern that
// silently matches nothing is a route that exists in the table and answers 404
// forever.
func ValidatePattern(pattern string) error {
	segs := splitSegments(pattern)
	var seen []string

	for _, seg := range segs {
		hasOpen := fmt.Contains(seg, "{")
		hasClose := fmt.Contains(seg, "}")

		if isParamSegment(seg) {
			name := seg[1 : len(seg)-1]
			if fmt.HasSuffix(name, "...") {
				return fmt.Errf(ErrMsgWildcardUnsupported, pattern)
			}
			if name == "" {
				return fmt.Errf(ErrMsgEmptyParamName, pattern)
			}
			if fmt.Contains(name, "{") || fmt.Contains(name, "}") {
				return fmt.Errf(ErrMsgMixedSegment, pattern, seg)
			}
			for _, s := range seen {
				if s == name {
					return fmt.Errf(ErrMsgDuplicateParamName, pattern, name)
				}
			}
			seen = append(seen, name)
		} else if hasOpen || hasClose {
			return fmt.Errf(ErrMsgMixedSegment, pattern, seg)
		}
	}
	return nil
}

// MatchPattern matches pathname against pattern. values holds one entry per
// name ParamNames(pattern) reports, in the same order. ok is false when the
// pattern does not match.
//
// A pattern with no parameters is matched by the pre-existing rule: trailing
// "/" matches the subtree, otherwise exact equality.
func MatchPattern(pattern, pathname string) (values []string, ok bool) {
	names := ParamNames(pattern)
	if len(names) == 0 {
		if fmt.HasSuffix(pattern, "/") {
			if fmt.HasPrefix(pathname, pattern) {
				return nil, true
			}
			return nil, false
		}
		if pattern == pathname {
			return nil, true
		}
		return nil, false
	}

	patSegs := splitSegments(pattern)
	pathSegs := splitSegments(pathname)

	if len(patSegs) != len(pathSegs) {
		return nil, false
	}

	vals := make([]string, 0, len(names))
	for i, patSeg := range patSegs {
		pathSeg := pathSegs[i]
		if isParamSegment(patSeg) {
			if pathSeg == "" {
				return nil, false
			}
			vals = append(vals, pathSeg)
		} else {
			if patSeg != pathSeg {
				return nil, false
			}
		}
	}

	return vals, true
}

// MoreSpecific reports whether a should win over b when both match the same
// path. See the ordering rule below.
func MoreSpecific(a, b string) bool {
	segsA := splitSegments(a)
	segsB := splitSegments(b)

	minLen := len(segsA)
	if len(segsB) < minLen {
		minLen = len(segsB)
	}

	for i := 0; i < minLen; i++ {
		paramA := isParamSegment(segsA[i])
		paramB := isParamSegment(segsB[i])
		if paramA != paramB {
			return !paramA // literal (paramA == false) beats parameter (paramB == true)
		}
	}

	if len(segsA) != len(segsB) {
		return len(segsA) > len(segsB)
	}

	if len(a) != len(b) {
		return len(a) > len(b)
	}

	return false
}

func isParamSegment(seg string) bool {
	return len(seg) >= 2 && fmt.HasPrefix(seg, "{") && fmt.HasSuffix(seg, "}")
}

func splitSegments(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	raw := fmt.Split(path, "/")
	var segs []string
	for _, s := range raw {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}
