package ioutils

import (
	"bytes"
	"encoding/json/jsontext"
	"io"
)

type jsonPathSegmentKind uint8

const (
	jsonPathSegmentKey jsonPathSegmentKind = iota
	jsonPathSegmentAnyKey
	jsonPathSegmentAnyIndex
)

// JsonPathSegment matches one member of an object or one element of an array.
type JsonPathSegment struct {
	kind jsonPathSegmentKind
	key  string
}

func NewJsonPathSegmentKey(name string) JsonPathSegment { return JsonPathSegment{key: name} }
func NewJsonPathSegmentAnyKey() JsonPathSegment         { return JsonPathSegment{kind: jsonPathSegmentAnyKey} }
func NewJsonPathSegmentAnyIndex() JsonPathSegment {
	return JsonPathSegment{kind: jsonPathSegmentAnyIndex}
}

type JsonStringRule struct {
	Path    []JsonPathSegment
	Rewrite func(value string) (replacement string, changed bool)
}

type jsonFrame struct {
	member    JsonPathSegment
	expectKey bool
}

type jsonRewriteReader struct {
	source      io.ReadCloser
	rules       []JsonStringRule
	keys        map[string]string
	maxKeyBytes int
	stack       []jsonFrame

	inString, escaped, readingKey bool
	rewrite                       func(string) (string, bool)
	raw, decoded                  []byte

	input  [4096]byte
	output []byte
	offset int
	err    error
}

// NewJsonRewriteReader rewrites string values in a valid JSON document.
// Paths start at the root. The first matching rule wins. Unchanged values
// and all surrounding bytes retain their original spelling. Only matched
// strings are buffered in full. The rules must remain unchanged during use.
func NewJsonRewriteReader(source io.ReadCloser, rules ...JsonStringRule) io.ReadCloser {
	if len(rules) == 0 {
		return source
	}
	r := &jsonRewriteReader{source: source, rules: rules, keys: make(map[string]string)}
	for _, rule := range rules {
		for _, segment := range rule.Path {
			if segment.kind == jsonPathSegmentKey {
				r.keys[segment.key] = segment.key
				// A byte of key text can occupy up to six bytes as a Unicode escape.
				r.maxKeyBytes = max(r.maxKeyBytes, 2+6*len(segment.key))
			}
		}
	}
	return r
}

func (r *jsonRewriteReader) Close() error { return r.source.Close() }

func (r *jsonRewriteReader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	for r.offset == len(r.output) {
		if r.err != nil {
			return 0, r.err
		}
		n, err := r.source.Read(r.input[:])
		r.output = r.output[:0]
		r.offset = 0
		r.err = err
		if err := r.scan(r.input[:n]); err != nil {
			r.err = err
		}
		if r.err == io.EOF && (r.inString || len(r.stack) > 0) {
			r.err = io.ErrUnexpectedEOF
		}
	}
	n := copy(dst, r.output[r.offset:])
	r.offset += n
	return n, nil
}

func (r *jsonRewriteReader) findStringRule() func(string) (string, bool) {
rules:
	for _, rule := range r.rules {
		if len(rule.Path) != len(r.stack) {
			continue
		}
		for i, segment := range rule.Path {
			member := r.stack[i].member
			if segment != member && !(segment.kind == jsonPathSegmentAnyKey && member.kind == jsonPathSegmentKey) {
				continue rules
			}
		}
		return rule.Rewrite
	}
	return nil
}

func (r *jsonRewriteReader) scan(data []byte) error {
	for len(data) > 0 {
		if r.inString {
			n, complete := r.scanStringPart(data)
			part := data[:n]
			data = data[n:]
			if r.rewrite == nil {
				r.output = append(r.output, part...)
			}
			if r.rewrite != nil || (r.readingKey && len(r.raw)+n <= r.maxKeyBytes) {
				r.raw = append(r.raw, part...)
			} else {
				r.readingKey = false
			}
			if complete {
				if err := r.finishString(); err != nil {
					return err
				}
			}
			continue
		}
		b := data[0]
		data = data[1:]
		switch b {
		case '{':
			r.stack = append(r.stack, jsonFrame{member: NewJsonPathSegmentAnyKey(), expectKey: true})
		case '[':
			r.stack = append(r.stack, jsonFrame{member: NewJsonPathSegmentAnyIndex()})
		case '}', ']':
			if len(r.stack) == 0 {
				return io.ErrUnexpectedEOF
			}
			r.stack = r.stack[:len(r.stack)-1]
		case ',':
			if len(r.stack) > 0 {
				f := &r.stack[len(r.stack)-1]
				f.expectKey = f.member.kind != jsonPathSegmentAnyIndex
			}
		case '"':
			r.inString = true
			r.raw = r.raw[:0]
			if len(r.stack) > 0 && r.stack[len(r.stack)-1].expectKey {
				f := &r.stack[len(r.stack)-1]
				f.expectKey = false
				f.member = NewJsonPathSegmentAnyKey() // unnamed member until an exact rule key is recognized
				r.readingKey = r.maxKeyBytes > 0
			} else {
				r.rewrite = r.findStringRule()
			}
			if r.readingKey || r.rewrite != nil {
				r.raw = append(r.raw, b)
			}
			if r.rewrite != nil {
				continue
			}
		}
		r.output = append(r.output, b)
	}
	return nil
}

// scanStringPart returns through the closing quote if it occurs in this chunk.
func (r *jsonRewriteReader) scanStringPart(data []byte) (int, bool) {
	start := 0
	for start < len(data) {
		i := bytes.IndexByte(data[start:], '"')
		if i < 0 {
			i = len(data)
		} else {
			i += start
		}
		j := i
		for j > start && data[j-1] == '\\' {
			j--
		}
		escaped := (i-j)%2 != 0
		if j == 0 && r.escaped {
			escaped = !escaped
		}
		if i == len(data) {
			r.escaped = escaped
			return i, false
		}
		r.escaped = false
		if !escaped {
			return i + 1, true
		}
		start = i + 1
	}
	return len(data), false
}

func (r *jsonRewriteReader) finishString() error {
	r.inString = false
	if !r.readingKey && r.rewrite == nil {
		return nil
	}
	value := r.raw[1 : len(r.raw)-1]
	if bytes.IndexByte(value, '\\') >= 0 {
		var err error
		r.decoded, err = jsontext.AppendUnquote(r.decoded[:0], r.raw)
		if err != nil {
			return err
		}
		value = r.decoded
	}
	if r.readingKey {
		if key, ok := r.keys[string(value)]; ok {
			r.stack[len(r.stack)-1].member = NewJsonPathSegmentKey(key)
		}
		r.readingKey = false
		return nil
	}
	replacement, changed := r.rewrite(string(value))
	r.rewrite = nil
	if !changed {
		r.output = append(r.output, r.raw...)
		return nil
	}
	var err error
	r.output, err = jsontext.AppendQuote(r.output, replacement)
	return err
}
