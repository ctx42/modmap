// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ctx42/ring/pkg/ring"
)

// wideLevel is the number of boxes on a level above which the map is too wide
// to be useful and the confirmation is asked for.
const wideLevel = 20

// confirm reports whether the map should be rendered. A map whose widest
// level holds no more than [wideLevel] boxes is always rendered. A wider one
// is announced on stderr and confirmed by the user, unless yes is set or
// there is nobody to ask, in which case it is rendered as well.
func confirm(
	ctx context.Context,
	rng *ring.Ring,
	widest int,
	yes bool,
) (bool, error) {

	if widest <= wideLevel {
		return true, nil
	}
	format := "" +
		"the widest level holds %d modules, " +
		"the map will be very wide\n"
	_, _ = fmt.Fprintf(rng.Stderr(), format, widest)
	if yes || !isTTY(rng.Stdin()) {
		return true, nil
	}
	return prompt(ctx, rng, "render it anyway? [y/N]: ")
}

// prompt writes the question to stderr and reads the answer from stdin. Only
// "y" and "yes", in any case, are taken for a yes. It returns the context
// error as soon as ctx is done, leaving the read of the answer behind.
func prompt(
	ctx context.Context,
	rng *ring.Ring,
	question string,
) (bool, error) {

	_, _ = fmt.Fprint(rng.Stderr(), question)
	type reply struct {
		line string
		err  error
	}
	rpls := make(chan reply, 1)
	go func() {
		line, err := bufio.NewReader(rng.Stdin()).ReadString('\n')
		rpls <- reply{line: line, err: err}
	}()
	var rpl reply
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case rpl = <-rpls:
	}
	if rpl.err != nil && !errors.Is(rpl.err, io.EOF) {
		return false, fmt.Errorf("read answer: %w", rpl.err)
	}
	answer := strings.ToLower(strings.TrimSpace(rpl.line))
	return answer == "y" || answer == "yes", nil
}

// isTTY reports whether the reader is a terminal.
func isTTY(src io.Reader) bool {
	fil, ok := src.(*os.File)
	if !ok {
		return false
	}
	inf, err := fil.Stat()
	if err != nil {
		return false
	}
	return inf.Mode()&os.ModeCharDevice != 0
}
