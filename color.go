package main

import (
	"io"
	"os"

	goocolor "github.com/gookit/color"
	"github.com/kubecolor/kubecolor/config"
	"github.com/kubecolor/kubecolor/kubectl"
	"github.com/kubecolor/kubecolor/printer"
	"github.com/mattn/go-isatty"
)

var colorTheme *config.Theme

func init() {
	// Only enable color when stdout is a terminal
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		goocolor.ForceColor()
	}

	v := config.NewViper()
	v.Set(config.PresetKey, string(config.PresetDark))
	cfg, err := config.Unmarshal(v)
	if err != nil {
		colorTheme = nil
		return
	}
	colorTheme = &cfg.Theme
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

	p := &printer.KubectlOutputColoredPrinter{
		SubcommandInfo: subInfo,
		Theme:          colorTheme,
	}
	p.Print(r, w)
	return true
}
