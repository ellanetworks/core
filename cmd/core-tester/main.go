package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"

	// Register all scenarios.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	coreN2Addresses []string
	gnbSpecs        []string
	gnbCoreTargets  []string
	verbose         bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "core-tester",
		Short: "Ella Core RAN/UE scenario runner",
	}

	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(runCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initLogger() {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	logger.Init(level)
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range scenarios.List() {
				fmt.Println(name)
			}

			return nil
		},
	}
}

// runCmd returns `core-tester run <scenario> [flags]`. Each registered
// scenario becomes a sub-command of run, so scenario-specific flags live on
// that sub-command and common flags are persistent on run itself.
func runCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run <scenario>",
		Short: "Run one scenario",
	}

	run.PersistentFlags().StringSliceVar(&coreN2Addresses, "ella-core-n2-address", nil,
		"Ella Core N2 SCTP address (repeatable)")
	run.PersistentFlags().StringArrayVar(&gnbSpecs, "gnb", nil,
		"gNB spec: <name>,n2=<addr>,n3=<addr>[,n3-secondary=<addr>] (repeatable)")
	run.PersistentFlags().StringArrayVar(&gnbCoreTargets, "gnb-core-target", nil,
		"pair <gnb-name>=<core-n2-addr> (repeatable)")
	run.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose logging")

	for _, name := range scenarios.List() {
		sc, _ := scenarios.Get(name)
		run.AddCommand(scenarioSubcommand(sc))
	}

	return run
}

func scenarioSubcommand(sc scenarios.Scenario) *cobra.Command {
	var params any

	cmd := &cobra.Command{
		Use:   sc.Name,
		Short: "Run scenario " + sc.Name,
		RunE: func(c *cobra.Command, args []string) error {
			initLogger()

			if len(coreN2Addresses) == 0 {
				return fmt.Errorf("at least one --ella-core-n2-address is required")
			}

			if len(gnbSpecs) == 0 {
				return fmt.Errorf("at least one --gnb is required")
			}

			gnbs, err := parseGNBs(gnbSpecs)
			if err != nil {
				return fmt.Errorf("invalid --gnb: %w", err)
			}

			targets, err := parseGNBCoreTargets(gnbCoreTargets)
			if err != nil {
				return fmt.Errorf("invalid --gnb-core-target: %w", err)
			}

			env := scenarios.Env{
				CoreN2Addresses: coreN2Addresses,
				GNBs:            gnbs,
				GNBCoreTargets:  targets,
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			if err := sc.Run(ctx, env, params); err != nil {
				logger.Logger.Error("scenario failed", zap.String("scenario", sc.Name), zap.Error(err))
				return err
			}

			return nil
		},
	}

	fs := pflag.NewFlagSet(sc.Name, pflag.ContinueOnError)
	params = sc.BindFlags(fs)
	cmd.Flags().AddFlagSet(fs)

	return cmd
}

func parseGNBs(specs []string) ([]scenarios.GNB, error) {
	out := make([]scenarios.GNB, 0, len(specs))

	for _, spec := range specs {
		parts := strings.Split(spec, ",")
		if len(parts) < 2 {
			return nil, fmt.Errorf("spec %q: need at least <name>,n2=<addr>", spec)
		}

		gnb := scenarios.GNB{Name: parts[0]}

		for _, kv := range parts[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return nil, fmt.Errorf("spec %q: expected k=v, got %q", spec, kv)
			}

			switch k {
			case "n2":
				gnb.N2Address = v
			case "n3":
				gnb.N3Address = v
			case "n3-secondary":
				gnb.N3Secondary = v
			default:
				return nil, fmt.Errorf("spec %q: unknown key %q", spec, k)
			}
		}

		if gnb.N2Address == "" {
			return nil, fmt.Errorf("spec %q: missing n2=<addr>", spec)
		}

		out = append(out, gnb)
	}

	return out, nil
}

func parseGNBCoreTargets(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	out := make(map[string]string, len(pairs))

	for _, p := range pairs {
		name, addr, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("pair %q: expected <gnb-name>=<addr>", p)
		}

		out[name] = addr
	}

	return out, nil
}
