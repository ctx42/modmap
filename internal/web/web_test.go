// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_New(t *testing.T) {
	t.Run("the browser opener defaults to the desktop one", func(t *testing.T) {
		// --- When ---
		have := New("ctx42", []byte("<svg/>"), nil, nil)

		// --- Then ---
		assert.Equal(t, "ctx42", have.title)
		assert.Equal(t, []byte("<svg/>"), have.doc)
		assert.NotNil(t, have.opener)
	})

	t.Run("the given opener is kept", func(t *testing.T) {
		// --- Given ---
		var opened string
		opener := func(url string) error { opened = url; return nil }

		// --- When ---
		have := New("ctx42", nil, nil, opener)

		// --- Then ---
		assert.NoError(t, have.opener("http://here/"))
		assert.Equal(t, "http://here/", opened)
	})
}

func Test_Server_Serve(t *testing.T) {
	t.Run("the map is served and the browser opened", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		urls := make(chan string, 1)
		opener := func(url string) error { urls <- url; return nil }
		srv := New("ctx42", []byte("<svg id=\"m\"/>"), logs(t), opener)
		done := make(chan error, 1)

		// --- When ---
		go func() { done <- srv.Serve(ctx, "127.0.0.1:0") }()

		// --- Then ---
		url := <-urls
		assert.Contain(t, "http://127.0.0.1:", url)

		body, ctype := get(t, url)
		assert.Contain(t, "text/html", ctype)
		assert.Contain(t, "<svg id=\"m\"/>", body)
		assert.Contain(t, "drag to pan", body)
		assert.Contain(t, "<title>ctx42</title>", body)

		body, ctype = get(t, url+"map.svg")
		assert.Equal(t, "image/svg+xml", ctype)
		assert.Equal(t, "<svg id=\"m\"/>", body)

		cancel()
		assert.NoError(t, <-done)
	})

	t.Run("an unknown path is not found", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		urls := make(chan string, 1)
		opener := func(url string) error { urls <- url; return nil }
		srv := New("ctx42", []byte("<svg/>"), logs(t), opener)
		done := make(chan error, 1)
		go func() { done <- srv.Serve(ctx, "127.0.0.1:0") }()
		url := <-urls

		// --- When ---
		//nolint:noctx,bodyclose // The body is closed below.
		rsp := must.Value(http.Get(url + "nope"))

		// --- Then ---
		assert.Equal(t, http.StatusNotFound, rsp.StatusCode)
		_ = rsp.Body.Close()

		cancel()
		assert.NoError(t, <-done)
	})

	t.Run("a browser which cannot be opened is only reported", func(t *testing.T) {
		// --- Given ---
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var lines []string
		logf := func(format string, args ...any) {
			lines = append(lines, format)
		}
		opener := func(string) error { cancel(); return ErrNoBrowser }
		srv := New("ctx42", []byte("<svg/>"), logf, opener)

		// --- When ---
		err := srv.Serve(ctx, "127.0.0.1:0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "open a browser at %s: %s", lines[len(lines)-1])
	})

	t.Run("error - the address cannot be listened on", func(t *testing.T) {
		// --- Given ---
		srv := New("ctx42", nil, logs(t), func(string) error { return nil })

		// --- When ---
		err := srv.Serve(t.Context(), "256.256.256.256:99999")

		// --- Then ---
		assert.ErrorContain(t, "listen on 256.256.256.256:99999", err)
	})
}

func Test_browseURL_tabular(t *testing.T) {
	tt := []struct {
		testN string

		addr string
		want string
	}{
		{"loopback", "127.0.0.1:8080", "http://127.0.0.1:8080/"},
		{"every interface", "[::]:8080", "http://127.0.0.1:8080/"},
		{"every IPv4 interface", "0.0.0.0:80", "http://127.0.0.1:80/"},
		{"a host", "example.com:80", "http://example.com:80/"},
		{"not an address", "pipe", "http://pipe/"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			addr := fakeAddr(tc.addr)

			// --- When ---
			have := browseURL(addr)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

// fakeAddr is a [net.Addr] reporting the string it is.
type fakeAddr string

func (adr fakeAddr) Network() string { return "tcp" }

func (adr fakeAddr) String() string { return string(adr) }

func Test_Server_handleMap(t *testing.T) {
	// --- Given ---
	srv := New("ctx42", []byte("<svg/>"), logs(t), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, pathMap, http.NoBody)

	// --- When ---
	srv.handleMap(rec, req)

	// --- Then ---
	assert.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<svg/>", rec.Body.String())
}

func Test_Server_html(t *testing.T) {
	// --- Given ---
	srv := New("the map", []byte("<svg id=\"m\"><g/></svg>"), logs(t), nil)

	// --- When ---
	have, err := srv.html()

	// --- Then ---
	assert.NoError(t, err)
	assert.Contain(t, "<title>the map</title>", string(have))
	assert.Contain(t, "<svg id=\"m\"><g/></svg>", string(have))
}

// logs returns a logger writing to the test log.
func logs(t *testing.T) func(format string, args ...any) {
	t.Helper()
	return func(format string, args ...any) { t.Logf(format, args...) }
}

// get returns the body and the content type of the URL.
func get(t *testing.T, url string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req := must.Value(
		http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody),
	)
	//nolint:bodyclose // The body is closed below.
	rsp := must.Value(http.DefaultClient.Do(req))
	defer func() { _ = rsp.Body.Close() }()
	return string(must.Value(io.ReadAll(rsp.Body))), rsp.Header.Get("Content-Type")
}
