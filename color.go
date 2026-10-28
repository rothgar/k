package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	goocolor "github.com/gookit/color"
	"github.com/kubecolor/kubecolor/config"
	"github.com/kubecolor/kubecolor/kubectl"
	"github.com/kubecolor/kubecolor/printer"
	"github.com/mattn/go-isatty"
)

var colorTheme *config.Theme

func init() {
	if !colorEnabled() {
		return
	}
	goocolor.ForceColor()

	v := config.NewViper()
	v.Set(config.PresetKey, string(config.PresetDark))
	cfg, err := config.Unmarshal(v, nil)
	if err != nil {
		colorTheme = nil
		return
	}
	colorTheme = &cfg.Theme
	colorTheme.ComputeCache()
}

// colorEnabled follows the same conventions as kubecolor: color when stdout
// is a terminal, never when NO_COLOR is set, and always when FORCE_COLOR is set.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// colorizeOutput takes kubectl args and pipes reader through kubecolor's
// printer to writer. Returns false if colorization is not supported for
// this command, in which case the caller should handle output directly.
func colorizeOutput(args []string, r io.Reader, w io.Writer) bool {
	if colorTheme == nil {
		return false
	}

	subInfo := kubectl.InspectSubcommandInfo(args, kubectl.NoopPluginHandler{})
	if !subInfo.SupportsColoring() {
		return false
	}

	var p printer.Printer
	switch {
	case subInfo.Subcommand == kubectl.Version && !subInfo.Help:
		// kubecolor's version printers inject a "Kubecolor Version" entry
		// into the output. k embeds kubecolor as a library so use our own.
		switch subInfo.Output {
		case kubectl.OutputJSON:
			p = &printer.JSONPrinter{Theme: colorTheme}
		case kubectl.OutputYAML:
			p = &printer.YAMLPrinter{Theme: colorTheme}
		default:
			p = &versionPrinter{Theme: colorTheme}
		}
	default:
		p = &printer.KubectlOutputColoredPrinter{
			SubcommandInfo: subInfo,
			Theme:          colorTheme,
		}
	}
	p.Print(r, w)
	return true
}

// versionPrinter colors the "Key: Value" lines of kubectl version output
// without appending anything to it.
type versionPrinter struct {
	Theme *config.Theme
}

func (p *versionPrinter) Print(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			fmt.Fprintln(w, line)
			continue
		}
		fmt.Fprintf(w, "%s: %s\n",
			printer.ColorDataKey(0, 2, p.Theme.Version.Key).Render(key),
			printer.ColorDataValue(val, p.Theme).Render(val),
		)
	}
}

// skipKVersionLine reports whether the "k version:" line should be omitted:
// for -o json / -o yaml (extra lines would corrupt the output) and for --help.
func skipKVersionLine(args []string) bool {
	subInfo := kubectl.InspectSubcommandInfo(args, kubectl.NoopPluginHandler{})
	return subInfo.Help || subInfo.Output == kubectl.OutputJSON || subInfo.Output == kubectl.OutputYAML
}
