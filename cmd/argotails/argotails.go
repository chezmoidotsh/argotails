package main

import (
	"os"

	"github.com/alecthomas/kong"

	ctrl "github.com/chezmoidotsh/argotails/internal/controller"
	zapcoreutils "github.com/chezmoidotsh/argotails/internal/zapcore"
)

func main() {
	cmd := ctrl.Command{} // trunk-ignore(golangci-lint/exhaustruct)
	ctx := kong.Parse(&cmd,
		kong.Name("argotails"),
		kong.Description("A Kubernetes controller to sychronize Tailscale devices with ArgoCD clusters."),
		kong.UsageOnError(),
		zapcoreutils.LevelEnablerMapper,
		zapcoreutils.EncoderMapper,
	)

	if err := ctx.Run(); err != nil {
		os.Exit(1)
	}
}
