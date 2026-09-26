package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/trungqwe/agents-coworkers/internal/workforce/control"
)

func main() {
	code := runCLI(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return control.ExitCodeUsageOrUnsupported
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "doctor":
		opts, exitCode, err := control.ParseDoctorFlags(subArgs, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "coworkers doctor: %v\n", err)
			return exitCode
		}
		_, code := control.RunDoctor(ctx, opts, stdout, stderr)
		return code

	case "attach":
		opts, exitCode, err := control.ParseAttachFlags(subArgs, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "coworkers attach: %v\n", err)
			return exitCode
		}
		_, code := control.RunAttach(ctx, opts, stdout, stderr)
		return code

	case "status":
		opts, exitCode, err := control.ParseStatusFlags(subArgs, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "coworkers status: %v\n", err)
			return exitCode
		}
		_, code := control.RunStatus(ctx, opts, stdout, stderr)
		return code

	case "run":
		fmt.Fprintf(stderr, "coworkers run is not supported in Slice 8A (requires Slice 8B autonomous engine)\n")
		return control.ExitCodeUsageOrUnsupported

	default:
		fmt.Fprintf(stderr, "coworkers: unknown subcommand %q\n", subcommand)
		printUsage(stderr)
		return control.ExitCodeUsageOrUnsupported
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: coworkers <command> [options]\n\n")
	fmt.Fprintf(w, "Commands:\n")
	fmt.Fprintf(w, "  doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]\n")
	fmt.Fprintf(w, "  attach --spec <run-spec.json> --session <id> --workspace <path> --manifest <output> [--ao-url <url>] [--gateway-url <url>] [--json]\n")
	fmt.Fprintf(w, "  status --manifest <run-manifest.json> [--ao-url <url>] [--gateway-url <url>] [--json]\n")
	fmt.Fprintf(w, "  run    (unsupported in Slice 8A; exits with code 1)\n")
}
