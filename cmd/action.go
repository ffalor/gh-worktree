package cmd

import (
	"github.com/ffalor/gh-wt/internal/config"
	"github.com/ffalor/gh-wt/internal/logger"
	"github.com/spf13/cobra"
)

var (
	listActionsFlag bool
	silentListFlag  bool
)

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage and list actions",
	RunE:  runAction,
}

func init() {
	rootCmd.AddCommand(actionCmd)
	actionCmd.Flags().BoolVarP(&listActionsFlag, "list", "l", false, "list all available actions")
	actionCmd.Flags().BoolVarP(&silentListFlag, "silent", "s", false, "suppress output when listing")
}

func runAction(cmd *cobra.Command, args []string) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	if listActionsFlag {
		// List all available actions
		if len(cfg.Actions) == 0 {
			if !silentListFlag {
				Log.Outf(logger.Yellow, "No actions configured.\n")
			}
			return nil
		}

		// Silent mode: just print action names, one per line
		if silentListFlag {
			for _, action := range cfg.Actions {
				Log.Outf(logger.Default, "%s\n", action.Name)
			}
			return nil
		}

		// Normal mode: print with formatting
		Log.Outf(logger.Default, "Available actions:\n")
		for _, action := range cfg.Actions {
			Log.Outf(logger.Default, "  - %s\n", action.Name)
		}
		return nil
	}

	// No flag provided, show help
	return cmd.Help()
}
