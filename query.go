package router

import (
	"webtyp.com/fmt"
)

const (
	queryStart    = '?'
	queryFragment = '#'
	querySep      = '&'
	queryEq       = '='
	queryPlus     = '+'
	queryEscape   = '%'
)

// QueryParam devuelve el valor de key en la query string de path, ya
// percent-decodificado. ok es false cuando la clave no aparece; un parámetro
// presente pero vacío ("?k=") devuelve "" con ok true — ausente y vacío son
// cosas distintas y el llamador debe poder distinguirlas.
//
// path es lo que devuelve Context.Path(): puede traer o no query string, y
// puede traer un fragmento. Todo lo que sigue al primer "#" se descarta antes
// de parsear: el fragmento nunca viaja al servidor y su presencia en un valor
// sólo puede venir de un llamador tratando de confundir a un validador.
//
// El valor NO se valida ni se sanea: es entrada del llamante como cualquier
// otra. Esta función sólo garantiza que lo que devuelve es lo que el llamante
// realmente envió — que es exactamente lo que un validador necesita para
// poder hacer su trabajo.
func QueryParam(path, key string) (value string, ok bool) {
	if idx := fmt.Index(path, string(queryFragment)); idx != -1 {
		path = path[:idx]
	}

	idxQ := fmt.Index(path, string(queryStart))
	if idxQ == -1 {
		return "", false
	}

	query := path[idxQ+1:]
	parts := fmt.Split(query, string(querySep))

	for _, part := range parts {
		var rawKey, rawVal string
		idxEq := fmt.Index(part, string(queryEq))
		if idxEq == -1 {
			rawKey = part
			rawVal = ""
		} else {
			rawKey = part[:idxEq]
			rawVal = part[idxEq+1:]
		}

		if QueryUnescape(rawKey) == key {
			return QueryUnescape(rawVal), true
		}
	}

	return "", false
}

// QueryUnescape decodifica una cadena percent-encoded de una query string:
// "%XX" pasa a su byte y "+" pasa a espacio. Una secuencia "%" malformada se
// deja tal cual — devolver un error obligaría a cada llamador a decidir qué
// hacer con un byte roto, y la respuesta correcta siempre es "trátalo como
// dato opaco y déjaselo al validador".
func QueryUnescape(s string) string {
	res := make([]byte, 0, len(s))
	i := 0
	n := len(s)

	for i < n {
		c := s[i]
		if c == queryPlus {
			res = append(res, ' ')
			i++
		} else if c == queryEscape && i+2 < n {
			b1, ok1 := unhex(s[i+1])
			b2, ok2 := unhex(s[i+2])
			if ok1 && ok2 {
				res = append(res, (b1<<4)|b2)
				i += 3
			} else {
				res = append(res, c)
				i++
			}
		} else {
			res = append(res, c)
			i++
		}
	}

	return string(res)
}

func unhex(c byte) (byte, bool) {
	if c >= '0' && c <= '9' {
		return c - '0', true
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10, true
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10, true
	}
	return 0, false
}
