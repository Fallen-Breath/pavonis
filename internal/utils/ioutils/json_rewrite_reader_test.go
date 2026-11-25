package ioutils

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestJSONRewritePaths(t *testing.T) {
	const input = `{ "versions": {"1.2.3": {"dependencies":{"tarball":"old"}, "d\u0069st":{"tarball":"o\u006cd"}}, "2":{"dist":{"tarball":123}}}, "files":[{"url":"one"},[{"url":"untouched"}]], "extra":{"dist":{"tarball":"old"}} }`
	const expected = `{ "versions": {"1.2.3": {"dependencies":{"tarball":"old"}, "d\u0069st":{"tarball":"new\"\n"}}, "2":{"dist":{"tarball":123}}}, "files":[{"url":"two"},[{"url":"untouched"}]], "extra":{"dist":{"tarball":"old"}} }`
	rules := []JsonStringRule{
		{[]JsonPathSegment{NewJsonPathSegmentKey("versions"), NewJsonPathSegmentAnyKey(), NewJsonPathSegmentKey("dist"), NewJsonPathSegmentKey("tarball")}, func(s string) (string, bool) {
			require.Equal(t, "old", s)
			return "new\"\n", true
		}},
		{[]JsonPathSegment{NewJsonPathSegmentKey("files"), NewJsonPathSegmentAnyIndex(), NewJsonPathSegmentKey("url")}, func(s string) (string, bool) {
			require.Equal(t, "one", s)
			return "two", true
		}},
	}
	for _, source := range []io.Reader{strings.NewReader(input), iotest.OneByteReader(strings.NewReader(input))} {
		r := NewJsonRewriteReader(io.NopCloser(source), rules...)
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, expected, string(out))
	}
}

func TestJSONRewriteWildcardMatchesArraysAndObjects(t *testing.T) {
	const input = `{"list":[{"url":"a"}],"map":{"v1":{"url":"b"}}}`
	wildcard := JsonPathSegment{kind: jsonPathSegmentWildcard}
	reader := NewJsonRewriteReader(io.NopCloser(strings.NewReader(input)), JsonStringRule{
		Path:    []JsonPathSegment{wildcard, wildcard, NewJsonPathSegmentKey("url")},
		Rewrite: func(value string) (string, bool) { return "/" + value, true },
	})
	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.JSONEq(t, `{"list":[{"url":"/a"}],"map":{"v1":{"url":"/b"}}}`, string(out))
}

func TestJSONRewriteRetainsOriginalString(t *testing.T) {
	const input = `["x\/y\u0021\\\"end","keep","*"]`
	var seen []string
	r := NewJsonRewriteReader(io.NopCloser(iotest.OneByteReader(strings.NewReader(input))), JsonStringRule{
		Path: []JsonPathSegment{NewJsonPathSegmentAnyIndex()},
		Rewrite: func(s string) (string, bool) {
			seen = append(seen, s)
			return "", false
		},
	})
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, input, string(out))
	require.Equal(t, []string{"x/y!\\\"end", "keep", "*"}, seen)
}

func TestJSONRewriteStreamsUnmatchedString(t *testing.T) {
	// The suffix is unavailable. A reader that waits for the entire key or
	// string before emitting it cannot return this prefix successfully.
	for _, prefix := range []string{`{"unmatched":"`, `{"`} {
		input := prefix + strings.Repeat("x", 16384)
		source := io.MultiReader(strings.NewReader(input), iotest.ErrReader(io.ErrClosedPipe))
		r := NewJsonRewriteReader(io.NopCloser(source), JsonStringRule{[]JsonPathSegment{NewJsonPathSegmentKey("k")}, func(string) (string, bool) {
			t.Fatal("unexpected match")
			return "", false
		}})
		out := make([]byte, len(input))
		_, err := io.ReadFull(r, out)
		require.NoError(t, err)
		require.Equal(t, input, string(out))
	}
}

func TestJSONRewriteFirstMatchingRuleWins(t *testing.T) {
	const input = `{"name":"original"}`
	firstCalled := false
	reader := NewJsonRewriteReader(io.NopCloser(strings.NewReader(input)),
		JsonStringRule{[]JsonPathSegment{NewJsonPathSegmentAnyKey()}, func(string) (string, bool) {
			firstCalled = true
			return "", false
		}},
		JsonStringRule{[]JsonPathSegment{NewJsonPathSegmentKey("name")}, func(string) (string, bool) {
			t.Fatal("later matching rule was called")
			return "", false
		}},
	)
	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.True(t, firstCalled)
	require.Equal(t, input, string(out))
}

func TestJSONRewriteRoot(t *testing.T) {
	r := NewJsonRewriteReader(io.NopCloser(strings.NewReader(` "root" `)), JsonStringRule{Rewrite: func(s string) (string, bool) { return s + "!", true }})
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, ` "root!" `, string(out))
}

func BenchmarkJSONRewrite(b *testing.B) {
	const upstream = "https://registry.example/"
	const mirror = "https://mirror.example/npm/"
	rewrite := func(s string) (string, bool) {
		path, ok := strings.CutPrefix(s, upstream)
		if !ok {
			return "", false
		}
		return mirror + path, true
	}
	rules := []JsonStringRule{{[]JsonPathSegment{NewJsonPathSegmentKey("versions"), NewJsonPathSegmentAnyKey(), NewJsonPathSegmentKey("dist"), NewJsonPathSegmentKey("tarball")}, rewrite}}
	literal := Replacement{Search: `"tarball":"` + upstream, Replace: `"tarball":"` + mirror}
	for _, fixture := range []struct {
		name                  string
		versions, readmeBytes int
	}{
		{"small", 5, 64}, {"many_versions", 4096, 128}, {"long_readme", 16, 1 << 20},
	} {
		var doc strings.Builder
		doc.WriteString(`{"versions":{`)
		for i := range fixture.versions {
			if i > 0 {
				doc.WriteByte(',')
			}
			fmt.Fprintf(&doc, `"%d":{"name":"sample","dist":{"tarball":"%ssample/-/sample-%d.tgz"},"dependencies":{"example":"^1"},"description":"%s"}`, i, upstream, i, strings.Repeat("x", 64))
		}
		fmt.Fprintf(&doc, `},"readme":"%s"}`, strings.Repeat("x", fixture.readmeBytes))
		data := []byte(doc.String())
		expected := bytes.ReplaceAll(data, []byte(literal.Search), []byte(literal.Replace))
		for _, impl := range []struct {
			name      string
			newReader func(io.ReadCloser) io.ReadCloser
		}{
			{"literal", func(r io.ReadCloser) io.ReadCloser { return NewRuleReplacingReader(r, literal) }},
			{"json", func(r io.ReadCloser) io.ReadCloser { return NewJsonRewriteReader(r, rules...) }},
		} {
			b.Run(fixture.name+"/"+impl.name, func(b *testing.B) {
				reader := func() io.ReadCloser { return impl.newReader(io.NopCloser(bytes.NewReader(data))) }
				probe := reader()
				got, err := io.ReadAll(probe)
				require.NoError(b, err)
				require.NoError(b, probe.Close())
				require.True(b, bytes.Equal(expected, got), "fixture output differs")
				b.ReportAllocs()
				b.SetBytes(int64(len(data)))
				for b.Loop() {
					r := reader()
					if _, err := io.Copy(io.Discard, r); err != nil {
						b.Fatal(err)
					}
					if err := r.Close(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
