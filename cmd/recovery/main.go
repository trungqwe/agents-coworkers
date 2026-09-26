package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: recovery <run|status> [flags]")
	}

	subcommand := args[0]
	switch subcommand {
	case "status":
		return runStatus(args[1:])
	case "run":
		return runRecovery(args[1:])
	case "cancel":
		return errors.New("cancel command is not supported via recovery CLI")
	default:
		return fmt.Errorf("unsupported command: %q (supported commands: run, status)", subcommand)
	}
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	var (
		checkpointPath string
		jsonOutput     bool
	)
	fs.StringVar(&checkpointPath, "checkpoint", "", "Path to checkpoint JSON file (required)")
	fs.BoolVar(&jsonOutput, "json", false, "Output status in JSON format")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if checkpointPath == "" {
		return errors.New("-checkpoint is required")
	}

	store := recovery.FileStore{Path: checkpointPath}
	cp, err := store.Load()
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}

	if jsonOutput {
		out, err := recovery.FormatStatusJSON(cp)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Println(recovery.FormatStatus(cp))
	}
	return nil
}

func runRecovery(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var (
		checkpointPath string
		workspace      string
		aoURL          string
		poll           time.Duration
		timeout        time.Duration
	)

	defaultAOURL := "http://127.0.0.1:3005"
	if env := os.Getenv("AO_BASE_URL"); env != "" {
		defaultAOURL = env
	}

	fs.StringVar(&checkpointPath, "checkpoint", "", "Path to checkpoint JSON file (required)")
	fs.StringVar(&workspace, "workspace", ".", "Path to workspace root")
	fs.StringVar(&aoURL, "ao-url", defaultAOURL, "AO base URL (default 'http://127.0.0.1:3005' or AO_BASE_URL)")
	fs.DurationVar(&poll, "poll", 500*time.Millisecond, "Poll interval (default 500ms)")
	fs.DurationVar(&timeout, "timeout", 0, "Optional total timeout (0 for no timeout)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if checkpointPath == "" {
		return errors.New("-checkpoint is required")
	}

	store := recovery.FileStore{Path: checkpointPath}
	cp, err := store.Load()
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}

	var lease recovery.Lease
	if cp.RunOwner != "" {
		fl, err := recovery.NewFileLease(workspace, cp.RunOwner)
		if err != nil {
			return fmt.Errorf("init lease: %w", err)
		}
		lease = fl
	}

	artifacts := recovery.GitArtifactReader{Root: workspace}

	aoClient, err := recovery.NewAOHTTPClient(aoURL, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("init AO client: %w", err)
	}

	dispatcher := &recovery.Dispatcher{
		Store:      store,
		AO:         aoClient,
		Artifacts:  artifacts,
		Lease:      lease,
		RetryDelay: poll,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		stepCP, stepErr := dispatcher.Step(ctx)
		if stepErr != nil {
			if errors.Is(stepErr, recovery.ErrLeaseHeld) || stepCP.TaskID == "" {
				return fmt.Errorf("recovery step failed: %w", stepErr)
			}
		}

		if stepCP.Terminal() {
			fmt.Println(recovery.FormatStatus(stepCP))
			if stepCP.State == recovery.StateBlocked || stepCP.State == recovery.StateCancelled || stepCP.State == recovery.StateCancelUnconfirmed {
				return fmt.Errorf("recovery terminated with state %s: %s", stepCP.State, recovery.SanitizeError(stepCP.LastErrorKind))
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}
