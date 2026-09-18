// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package web serves a rendered map over HTTP and opens it in a browser, so
// a map can be looked at without being written to disk. The page it serves
// wraps the map in a pan and zoom viewer; the map itself stays the same
// script-free SVG the renderer produces, downloadable from "/map.svg".
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"
)

// Server timeouts.
const (
	// headerTimeout is how long a client has to send its request headers.
	headerTimeout = 5 * time.Second

	// closeTimeout is how long the in-flight requests have to finish once
	// the server is asked to stop.
	closeTimeout = 5 * time.Second
)

// Paths the server answers on.
const (
	pathPage = "/"        // The viewer page.
	pathMap  = "/map.svg" // The map itself.
)

//go:embed page.html
var pages embed.FS

// page is the viewer page template.
var page = template.Must(template.ParseFS(pages, "page.html"))

// Server serves one rendered map over HTTP.
type Server struct {
	title  string                           // Name the map is shown under.
	doc    []byte                           // The rendered SVG document.
	logf   func(format string, args ...any) // Progress reporting.
	opener func(url string) error           // Opens the URL in a browser.
}

// New returns the server showing the map document under the given title.
// A nil opener means the desktop browser; tests pass their own.
func New(
	title string,
	doc []byte,
	logf func(format string, args ...any),
	opener func(url string) error,
) *Server {

	if opener == nil {
		opener = open
	}
	return &Server{
		title:  title,
		doc:    doc,
		logf:   logf,
		opener: opener,
	}
}

// Serve listens on addr, opens the map in a browser, and keeps serving until
// ctx is done. The address is a host:port pair; a zero port picks a free one.
// It returns the listen error without opening anything when addr is taken.
func (srv *Server) Serve(ctx context.Context, addr string) error {
	var lcf net.ListenConfig
	lst, err := lcf.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	url := browseURL(lst.Addr())
	srv.logf("serving %s on %s", srv.title, url)
	srv.logf("press Ctrl-C to stop")

	mux := http.NewServeMux()
	mux.HandleFunc(pathPage, srv.handlePage)
	mux.HandleFunc(pathMap, srv.handleMap)
	hsr := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: headerTimeout,
	}

	if err = srv.opener(url); err != nil {
		srv.logf("open a browser at %s: %s", url, err)
	}

	done := make(chan error, 1)
	go func() { done <- hsr.Serve(lst) }()

	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve map: %w", err)

	case <-ctx.Done():
		stop, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			closeTimeout,
		)
		defer cancel()
		_ = hsr.Shutdown(stop)
		return nil
	}
}

// browseURL returns the URL the map is opened at. A listener on every
// interface reports an address no browser can be pointed at, so the
// loopback stands in for it.
func browseURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String() + pathPage
	}
	ip := net.ParseIP(host)
	if host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + pathPage
}

// handlePage writes the viewer page with the map inlined in it.
func (srv *Server) handlePage(wrt http.ResponseWriter, req *http.Request) {
	if req.URL.Path != pathPage {
		http.NotFound(wrt, req)
		return
	}
	doc, err := srv.html()
	if err != nil {
		http.Error(wrt, err.Error(), http.StatusInternalServerError)
		return
	}
	wrt.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = wrt.Write(doc)
}

// handleMap writes the map document on its own, so it can be saved.
func (srv *Server) handleMap(wrt http.ResponseWriter, _ *http.Request) {
	wrt.Header().Set("Content-Type", "image/svg+xml")
	_, _ = wrt.Write(srv.doc)
}

// html renders the viewer page with the map inlined in it.
func (srv *Server) html() ([]byte, error) {
	data := struct {
		Title string
		Map   template.HTML
	}{
		Title: srv.title,
		Map:   template.HTML(srv.doc), //nolint:gosec
	}
	buf := &bytes.Buffer{}
	if err := page.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("render page: %w", err)
	}
	return buf.Bytes(), nil
}
