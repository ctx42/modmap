// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/conf"
	"github.com/ctx42/modmap/internal/svg"
	"github.com/ctx42/modmap/internal/view"
	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
)

// run generates the image the configuration asks for: the map described by
// the options, or the maps the configuration file selects, side by side.
func run(ctx context.Context, rng *ring.Ring, cfg *config) error {
	if cfg.conf == "" {
		spc := spec{
			Dirs:   cfg.roots,
			Filter: mod.NewFilter(cfg.include, cfg.exclude),
			Out:    cfg.out,
		}
		return generate(ctx, rng, cfg, spc)
	}

	fil, err := conf.Load(cfg.conf)
	if err != nil {
		return err
	}
	maps, err := fil.Select(cfg.names)
	if err != nil {
		return err
	}
	return generate(ctx, rng, cfg, combine(fil.Out, maps))
}

// combine returns the spec drawing the maps side by side, one column per map
// in the order given, into the file at out. The directories of every map are
// scanned together and a module is kept when any of the maps keeps it, so a
// requirement crossing maps is drawn as well.
func combine(out string, maps []conf.Map) spec {
	spc := spec{Out: out}
	names := make([]string, 0, len(maps))
	flts := make([]mod.Filter, 0, len(maps))
	seen := make(map[string]bool)
	for _, mp := range maps {
		flt := mod.NewFilter(mp.Include, mp.Exclude)
		names = append(names, mp.Name)
		flts = append(flts, flt)
		col := svg.Column{Name: mp.Name, Filter: flt}
		spc.Columns = append(spc.Columns, col)
		for _, dir := range mp.Dirs {
			if !seen[dir] {
				seen[dir] = true
				spc.Dirs = append(spc.Dirs, dir)
			}
		}
	}
	spc.Name = strings.Join(names, "-")
	spc.Filter = mod.AnyOf(flts...)
	return spc
}

// spec describes one image to generate.
type spec struct {
	// Name of the image, empty for the map described by the options.
	Name string

	// Dirs holds the absolute paths of the directories to scan.
	Dirs []string

	// Filter decides which modules are rendered.
	Filter mod.Filter

	// Columns holds the maps drawn side by side, left to right. Without
	// columns every module is drawn in one unnamed column.
	Columns []svg.Column

	// Out is the absolute path of the SVG file to write. It is ignored
	// when the image is opened in a browser instead.
	Out string
}

// title returns the name the image is shown under in a browser.
func (spc spec) title() string {
	if spc.Name != "" {
		return spc.Name
	}
	if len(spc.Dirs) > 0 {
		return filepath.Base(spc.Dirs[0])
	}
	return binName
}

// generate builds the image described by spc and either writes it to its
// output file or opens it in a browser. It returns nil without writing
// anything when the user declines to render an image with a very wide level.
func generate(
	ctx context.Context,
	rng *ring.Ring,
	cfg *config,
	spc spec,
) error {

	logf := func(format string, args ...any) {
		_, _ = fmt.Fprintf(rng.Stderr(), format+"\n", args...)
	}
	mods, err := mod.NewScanner(spc.Filter, logf).Scan(ctx, spc.Dirs)
	if err != nil {
		return err
	}

	rsv, err := mod.NewResolver(spc.Filter, rng.EnvAll(), logf)
	if err != nil {
		return err
	}
	defer func() { _ = rsv.Close() }()
	if err = rsv.Resolve(ctx, mods); err != nil {
		return err
	}

	grp, err := graph.New(mods)
	if err != nil {
		return err
	}
	logf("graph has %d modules, widest level %d", grp.Len(), grp.Widest())

	render, err := confirm(ctx, rng, grp.Widest(), cfg.yes)
	if err != nil {
		return err
	}
	if !render {
		return nil
	}
	if cfg.web {
		return show(logf, cfg, spc, grp)
	}
	return write(grp, spc.Columns, spc.Out)
}

// show renders the graph into a page under the system temporary directory
// and opens it in a browser. The page outlives the run, so its path is
// reported whether or not the browser could be opened.
func show(
	logf func(format string, args ...any),
	cfg *config,
	spc spec,
	grp *graph.Graph,
) error {

	rnd, err := svg.NewRenderer()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err = rnd.Render(grp, spc.Columns, buf); err != nil {
		return err
	}
	pth, err := view.Open(spc.title(), buf.Bytes(), cfg.opener)
	if pth != "" {
		logf("map written to %s", pth)
	}
	return err
}

// write renders the graph, drawn in the columns, into the file at pth. The
// image is rendered in full before the file is replaced, so a failure leaves
// the previous image intact.
func write(grp *graph.Graph, cols []svg.Column, pth string) error {
	rnd, err := svg.NewRenderer()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err = rnd.Render(grp, cols, buf); err != nil {
		return err
	}
	if err = replaceFile(pth, buf.Bytes()); err != nil {
		return fmt.Errorf("write map file: %w", err)
	}
	return nil
}
