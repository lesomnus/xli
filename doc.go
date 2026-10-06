package xli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
)

// WriteMarkdown writes a Markdown reference for c and all of its visible
// subcommands: one section per command with its usage, description,
// arguments, options, and subcommands. Hidden commands and flags are left out.
func WriteMarkdown(w io.Writer, c *Command) error {
	p := &docPrinter{w: w}
	walkDoc(c, func(c *Command, depth int) {
		path := commandPath(c)
		if depth == 0 {
			p.printf("# %s\n\n", path)
		} else {
			p.printf("## %s\n\n", path)
		}
		if c.Brief != "" {
			p.printf("%s\n\n", c.Brief)
		}
		p.printf("```\n%s\n```\n\n", usageLine(c))
		if c.Synop != "" {
			p.printf("%s\n\n", c.Synop)
		}

		if len(c.Args) > 0 {
			p.printf("**Arguments**\n\n| Argument | Description |\n| --- | --- |\n")
			for _, a := range c.Args {
				info := a.Info()
				p.printf("| `%s` | %s |\n", mdCell(info.Usage.String()), mdCell(argDesc(info)))
			}
			p.printf("\n")
		}

		if fs := c.Flags.Visible(); len(fs) > 0 {
			p.printf("**Options**\n\n| Option | Type | Description |\n| --- | --- | --- |\n")
			for _, f := range fs {
				info := f.Info()
				typ := ""
				if info.Type != "" {
					typ = fmt.Sprintf("`%s`", mdCell(info.Type))
				}
				p.printf("| `%s` | %s | %s |\n", flagLabel(info), typ, mdCell(flagDesc(info)))
			}
			p.printf("\n")
		}

		if cs := c.Commands.Visible(); len(cs) > 0 {
			p.printf("**Commands**\n\n| Command | Description |\n| --- | --- |\n")
			for _, sub := range cs {
				p.printf("| [`%s`](#%s) | %s |\n", sub.Name, mdAnchor(commandPath(sub)), mdCell(sub.Brief))
			}
			p.printf("\n")
		}
	})
	return p.err
}

// WriteMan writes a man page in roff format for c, documenting c and all of
// its visible subcommands. section is the manual section, usually 1.
// Hidden commands and flags are left out.
func WriteMan(w io.Writer, c *Command, section int) error {
	p := &docPrinter{w: w}
	has_commands := false
	walkDoc(c, func(c *Command, depth int) {
		if depth == 0 {
			p.printf(".TH %s %d\n", roff(strings.ToUpper(c.Name)), section)
			p.printf(".SH NAME\n%s", roff(c.Name))
			if c.Brief != "" {
				p.printf(" \\- %s", roff(c.Brief))
			}
			p.printf("\n.SH SYNOPSIS\n%s\n", roff(usageLine(c)))
			if c.Synop != "" {
				p.printf(".SH DESCRIPTION\n%s\n", roff(c.Synop))
			}
		} else {
			if !has_commands {
				p.printf(".SH COMMANDS\n")
				has_commands = true
			}
			p.printf(".SS %s\n", roff(commandPath(c)))
			if c.Brief != "" {
				p.printf("%s\n.PP\n", roff(c.Brief))
			}
			p.printf(".B %s\n", roff(usageLine(c)))
			if c.Synop != "" {
				p.printf(".PP\n%s\n", roff(c.Synop))
			}
		}

		if len(c.Args) > 0 {
			if depth == 0 {
				p.printf(".SH ARGUMENTS\n")
			} else {
				p.printf(".PP\nArguments:\n")
			}
			for _, a := range c.Args {
				info := a.Info()
				p.printf(".TP\n.B %s\n%s\n", roff(info.Usage.String()), roff(argDesc(info)))
			}
		}

		if fs := c.Flags.Visible(); len(fs) > 0 {
			if depth == 0 {
				p.printf(".SH OPTIONS\n")
			} else {
				p.printf(".PP\nOptions:\n")
			}
			for _, f := range fs {
				info := f.Info()
				p.printf(".TP\n.B %s", roff(flagLabel(info)))
				if info.Type != "" {
					p.printf(" \" %s\"", roff(info.Type))
				}
				p.printf("\n%s\n", roff(flagDesc(info)))
			}
		}
	})
	return p.err
}

// walkDoc visits c and its visible descendants depth-first (c has depth 0),
// linking parents on the way so usage lines and command paths are complete.
func walkDoc(c *Command, f func(c *Command, depth int)) {
	var walk func(c *Command, depth int)
	walk = func(c *Command, depth int) {
		subs := c.Commands.Visible()
		for _, sub := range subs {
			sub.parent = c
		}
		f(c, depth)
		for _, sub := range subs {
			walk(sub, depth+1)
		}
	}
	walk(c, 0)
}

type docPrinter struct {
	w   io.Writer
	err error
}

func (p *docPrinter) printf(format string, vs ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, vs...)
}

func commandPath(c *Command) string {
	tree := c.Tree()
	names := make([]string, len(tree))
	for i, v := range tree {
		names[i] = v.Name
	}
	return strings.Join(names, " ")
}

// flagLabel renders "-v, --verbose" or "--verbose".
func flagLabel(info *flg.Info) string {
	if info.Alias == 0 {
		return "--" + info.Name
	}
	return fmt.Sprintf("-%c, --%s", info.Alias, info.Name)
}

func flagDesc(info *flg.Info) string {
	vs := []string{}
	if info.Brief != "" {
		vs = append(vs, info.Brief)
	}
	if info.Required {
		vs = append(vs, "(required)")
	}
	if info.HasDefault {
		vs = append(vs, fmt.Sprintf("(default: %s)", info.Default))
	}
	return strings.Join(vs, " ")
}

func argDesc(info *arg.Info) string {
	vs := []string{}
	if info.Brief != "" {
		vs = append(vs, info.Brief)
	}
	if info.HasDefault {
		vs = append(vs, fmt.Sprintf("(default: %s)", info.Default))
	}
	return strings.Join(vs, " ")
}

// mdCell escapes text for a Markdown table cell.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", " ")
}

// mdAnchor returns the GitHub-style heading anchor for a command path.
func mdAnchor(path string) string {
	b := strings.Builder{}
	for _, r := range strings.ToLower(path) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || ('a' <= r && r <= 'z') || ('0' <= r && r <= '9'):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// roff escapes text for a roff (man) document.
func roff(s string) string {
	s = strings.ReplaceAll(s, `\`, `\e`)
	s = strings.ReplaceAll(s, "-", `\-`)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, ".") || strings.HasPrefix(l, "'") {
			lines[i] = `\&` + l
		}
	}
	return strings.Join(lines, "\n")
}
