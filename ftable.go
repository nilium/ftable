// Command ftable reads all text from stdin and passes it through a text/tabwriter to produce pretty columnar
// output. It will optionally wrap all output in box drawing glyphs if the -box flag is set, with a header row
// when -header is passed in addition to -box.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

const termChar byte = 0x0e

type tabFlags uint

var flagBits = map[uint]string{
	tabwriter.FilterHTML:          "filter-html",
	tabwriter.StripEscape:         "strip-escape",
	tabwriter.AlignRight:          "align-right",
	tabwriter.DiscardEmptyColumns: "discard-empty",
	tabwriter.TabIndent:           "tab-indent",
	tabwriter.Debug:               "debug",
}

var flagNames = map[string]uint{}

func init() {
	for b, n := range flagBits {
		flagNames[n] = b
	}
}

func (t *tabFlags) Set(v string) error {
	flags := uint(*t)
	for _, name := range strings.Split(v, ",") {
		if b, ok := flagNames[v]; ok {
			flags |= b
		} else {
			return fmt.Errorf("unrecognized flag %q", name)
		}
	}
	*t = tabFlags(flags)
	return nil
}

func (t *tabFlags) String() string {
	flags := []string{}
	ui := uint(*t)
	for b, n := range flagBits {
		if ui&b == b {
			flags = append(flags, n)
		}
	}
	sort.Strings(flags)
	return strings.Join(flags, ",")
}

func main() {
	var mwidth, tabwidth, padding int
	var padchar string
	var flags tabFlags
	var box, header, rowlines bool

	flag.BoolVar(&box, "box", false, "whether to box the output with box-drawing characters")
	flag.BoolVar(&header, "header", false, "whether the first line of boxed output is a header box")
	flag.BoolVar(&rowlines, "rowlines", false, "whether to insert row separators in box mode")
	flag.IntVar(&mwidth, "minwidth", 0, "the minimum `width` of a column in bytes")
	flag.IntVar(&tabwidth, "tabwidth", 8, "the `width` of a tab in bytes")
	flag.IntVar(&padding, "padding", 1, "`padding`")
	flag.StringVar(&padchar, "padchar", " ", "the padding `char` to use; only the first byte is used if a multibyte string is provided")
	flag.Var(&flags, "flags", "any comma-separated combination of the flags: filter-html, strip-escape, align-right, discard-empty, tab-indent, debug")
	flag.Parse()

	if len(padchar) != 1 {
		fmt.Fprintf(os.Stderr, "invalid padchar of length %d", len(padchar))
		os.Exit(1)
	}

	if !box {
		w := tabwriter.NewWriter(os.Stdout, mwidth, tabwidth, padding, padchar[0], uint(flags))
		defer w.Flush()

		if _, err := io.Copy(w, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "error reading from stdin: %v", err)
			os.Exit(1)
		}
		return
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "error reading from stdin: %v", err)
		os.Exit(1)
	}

	bs := buf.Bytes()
	if tab := []byte("\t"); uint(flags)&tabwriter.AlignRight == tabwriter.AlignRight {
		bs = bytes.Replace(bs, tab, []byte{' ', termChar, ' ', '\t'}, -1)
	} else {
		bs = bytes.Replace(bs, tab, []byte{'\t', termChar, ' '}, -1)
	}

	var outbuf bytes.Buffer
	w := tabwriter.NewWriter(&outbuf, mwidth, tabwidth, padding, padchar[0], uint(flags))
	w.Write(bs)
	w.Flush()

	var lines = bytes.Split(outbuf.Bytes(), []byte("\n"))
	maxLen := 0
	separators := map[int]struct{}{}
	for _, bs := range lines {
		if reallen := len(bytes.Runes(bs)); maxLen < reallen {
			maxLen = reallen
		}

		for i, r := range bytes.Runes(bs) {
			if r == rune(termChar) {
				separators[i] = struct{}{}
			}
		}
	}

	applySep := func(sep []byte, r rune, force bool) []byte {
		runes := bytes.Runes(sep)

		for i := range separators {
			if len(runes) > i && (force || runes[i] == rune(termChar)) {
				runes[i] = r
			}
		}

		return []byte(string(runes))
	}

	maxLen++

	var sepLine []byte
	if rowlines {
		sepLine = []byte(fmt.Sprintf("├─%s┤\n", applySep(bytes.Repeat([]byte("─"), maxLen), '┼', true)))
	}

	for n, line := range lines {
		if n == 0 {
			if header {
				line = applySep(line, '┃', false)
				if rl := len(bytes.Runes(line)); rl < maxLen {
					line = append(line, bytes.Repeat([]byte{' '}, maxLen-rl)...)
				}

				sep := bytes.Repeat([]byte("━"), maxLen)
				fmt.Printf("┏━%s┓\n", applySep(sep, '┳', true))
				fmt.Printf("┃ %s┃\n", applySep(line, '┃', false))
				fmt.Printf("┡━%s┩\n", applySep(sep, '╇', true))
			} else {
				line = applySep(line, '│', false)
				if rl := len(bytes.Runes(line)); rl < maxLen {
					line = append(line, bytes.Repeat([]byte{' '}, maxLen-rl)...)
				}

				sep := bytes.Repeat([]byte("─"), maxLen)
				fmt.Printf("┌─%s┐\n", applySep(sep, '┬', true))
				fmt.Printf("│ %s│\n", line)
			}
		} else {
			line = applySep(line, '│', false)
			if rl := len(bytes.Runes(line)); rl < maxLen {
				line = append(line, bytes.Repeat([]byte{' '}, maxLen-rl)...)
			}

			if rowlines && ((header && n > 1) || !header) {
				os.Stdout.Write(sepLine)
			}
			fmt.Printf("│ %s│\n", line)
		}

	}
	fmt.Printf("└━%s┘\n", applySep(bytes.Repeat([]byte("━"), maxLen), '┴', true))

}
