// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package view writes a rendered map as a page opened straight from disk
// and hands it to a browser, so a map can be looked at without a server
// and without being kept anywhere the user has to name. The page wraps the
// map in a pan and zoom viewer with the SVG inlined in it; the map itself
// is written beside the page as the same script-free SVG the renderer
// produces, linked for saving.
package view

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Names the viewer writes under.
const (
	// dirName is the directory, under the system temporary directory,
	// holding the pages. Every run rewrites the files of the map it
	// shows, so the pages never pile up.
	dirName = "modmap"

	// defName is the file name of a map whose title carries nothing a
	// file name can be built from.
	defName = "map"
)

// Permissions the viewer writes with. The pages are the user's alone.
const (
	// dirPerm is the mode of the directory holding the pages.
	dirPerm = 0o700

	// filePerm is the mode of the page and the map file.
	filePerm = 0o600
)

// ErrUnsafeDir is returned when the directory holding the pages is not a
// directory of the current user's own.
var ErrUnsafeDir = errors.New("unsafe map directory")

//go:embed page.html
var pages embed.FS

// page is the viewer page template.
var page = template.Must(template.ParseFS(pages, "page.html"))

// Open writes the map document as a viewer page shown under the given
// title and opens it in a browser. It returns the path of the page, which
// outlives the run because the browser reads it once modmap is gone; the
// path is returned even when the browser cannot be opened, so the user is
// told where the map is. A nil opener means the desktop browser; tests
// pass their own.
func Open(
	title string,
	doc []byte,
	opener func(url string) error,
) (string, error) {

	pth, err := write(filepath.Join(os.TempDir(), dirName), title, doc)
	if err != nil {
		return "", err
	}
	if opener == nil {
		opener = open
	}
	if err = opener(fileURL(pth)); err != nil {
		return pth, fmt.Errorf("open a browser: %w", err)
	}
	return pth, nil
}

// write writes the viewer page and the map it shows into dir, creating the
// directory when it is not there. It returns the path of the page.
func write(dir, title string, doc []byte) (string, error) {
	if err := privateDir(dir); err != nil {
		return "", err
	}
	name := slug(title)
	htm, err := html(title, name+".svg", doc)
	if err != nil {
		return "", err
	}
	if err = replaceFile(dir, name+".svg", doc); err != nil {
		return "", fmt.Errorf("write map file: %w", err)
	}
	if err = replaceFile(dir, name+".html", htm); err != nil {
		return "", fmt.Errorf("write map page: %w", err)
	}
	return filepath.Join(dir, name+".html"), nil
}

// privateDir creates dir when it is not there and makes sure it is a
// directory only the current user can reach. It returns [ErrUnsafeDir] for a
// symbolic link, a non-directory, or a directory of another user.
func privateDir(dir string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create map directory: %w", err)
	}
	inf, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("create map directory: %w", err)
	}
	if !inf.IsDir() {
		return fmt.Errorf("%w: not a directory: %s", ErrUnsafeDir, dir)
	}
	if !ownedByUser(inf) {
		return fmt.Errorf("%w: owned by another user: %s", ErrUnsafeDir, dir)
	}
	if inf.Mode().Perm() == dirPerm {
		return nil
	}
	if err = os.Chmod(dir, dirPerm); err != nil {
		return fmt.Errorf("create map directory: %w", err)
	}
	return nil
}

// replaceFile writes data to the named file in dir through a temporary file
// renamed over it, so a reader never sees a partial file and a symbolic link
// in its place is replaced rather than followed.
func replaceFile(dir, name string, data []byte) error {
	tmp, err := os.CreateTemp(dir, name+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), filePerm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}

// html renders the viewer page showing the map under the given title, with
// the map inlined in it and the map file linked for saving.
func html(title, link string, doc []byte) ([]byte, error) {
	data := struct {
		Title string
		Link  string
		Map   template.HTML
	}{
		Title: title,
		Link:  link,
		Map:   template.HTML(doc), //nolint:gosec
	}
	buf := &bytes.Buffer{}
	if err := page.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("render page: %w", err)
	}
	return buf.Bytes(), nil
}

// slug returns the base name the map is written under: the title with
// every character a file name should not carry replaced by a dash. It
// returns defName when nothing usable is left.
func slug(title string) string {
	keep := func(rne rune) rune {
		switch {
		case rne >= 'a' && rne <= 'z',
			rne >= 'A' && rne <= 'Z',
			rne >= '0' && rne <= '9',
			rne == '_':
			return rne

		default:
			return '-'
		}
	}
	name := strings.Trim(strings.Map(keep, title), "-")
	if name == "" {
		return defName
	}
	return name
}

// fileURL returns the "file" URL of the path, which is what a browser is
// pointed at whatever the operating system spells paths like.
func fileURL(pth string) string {
	pth = filepath.ToSlash(pth)
	if !strings.HasPrefix(pth, "/") {
		pth = "/" + pth
	}
	uri := url.URL{Scheme: "file", Path: pth}
	return uri.String()
}
