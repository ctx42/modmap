// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/conf"
	"github.com/ctx42/modmap/internal/svg"
	"github.com/ctx42/modmap/internal/web"
	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
)

// run generates every map the configuration asks for: the one described by
// the options, or every map the configuration file declares.
func run(ctx context.Context, rng *ring.Ring, cfg *config) error {
	if cfg.conf == "" {
		spc := spec{
			Dirs:    cfg.roots,
			Include: cfg.include,
			Exclude: cfg.exclude,
			Out:     cfg.out,
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
	for _, mp := range maps {
		spc := spec{
			Name:    mp.Name,
			Dirs:    mp.Dirs,
			Include: mp.Include,
			Exclude: mp.Exclude,
			Out:     mp.Out,
		}
		if err = generate(ctx, rng, cfg, spc); err != nil {
			return fmt.Errorf("map %s: %w", mp.Name, err)
		}
	}
	return nil
}

// spec describes one map to generate.
type spec struct {
	// Name of the map, empty for the map described by the options.
	Name string

	// Dirs holds the absolute paths of the directories to scan.
	Dirs []string

	// Include holds the module path globs deciding which modules are
	// rendered. An empty list renders every module found.
	Include []string

	// Exclude holds the module path globs removing modules from the map.
	Exclude []string

	// Out is the absolute path of the SVG file to write. It is empty
	// when the map is served instead of written.
	Out string
}

// title returns the name the map is shown under when it is served.
func (spc spec) title() string {
	if spc.Name != "" {
		return spc.Name
	}
	if len(spc.Dirs) > 0 {
		return filepath.Base(spc.Dirs[0])
	}
	return binName
}

// generate builds the map described by spc and either writes it to its
// output file or serves it in a browser. It returns nil without writing
// anything when the user declines to render a map with a very wide level.
func generate(
	ctx context.Context,
	rng *ring.Ring,
	cfg *config,
	spc spec,
) error {

	logf := func(format string, args ...any) {
		_, _ = fmt.Fprintf(rng.Stderr(), format+"\n", args...)
	}
	flt := mod.NewFilter(spc.Include, spc.Exclude)
	mods, err := mod.NewScanner(flt, logf).Scan(spc.Dirs)
	if err != nil {
		return err
	}

	rsv, err := mod.NewResolver(flt, rng.EnvAll(), logf)
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

	render, err := confirm(rng, grp.Widest(), cfg.yes)
	if err != nil {
		return err
	}
	if !render {
		return nil
	}
	if cfg.web {
		return serve(ctx, logf, cfg, spc.title(), grp)
	}
	return write(grp, spc.Out)
}

// serve renders the graph and serves it in a browser until ctx is done.
func serve(
	ctx context.Context,
	logf func(format string, args ...any),
	cfg *config,
	title string,
	grp *graph.Graph,
) error {

	rnd, err := svg.NewRenderer()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err = rnd.Render(grp, buf); err != nil {
		return err
	}
	srv := web.New(title, buf.Bytes(), logf, cfg.opener)
	return srv.Serve(ctx, cfg.webAddr)
}

// write renders the graph into the file at pth.
func write(grp *graph.Graph, pth string) error {
	rnd, err := svg.NewRenderer()
	if err != nil {
		return err
	}
	fil, err := os.Create(pth) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create map file: %w", err)
	}
	if err = rnd.Render(grp, fil); err != nil {
		_ = fil.Close()
		return err
	}
	if err = fil.Close(); err != nil {
		return fmt.Errorf("close map file: %w", err)
	}
	return nil
}
