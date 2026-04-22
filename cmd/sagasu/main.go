package main

import (
	"fmt"
	"github.com/shinyonogi/sagasu/internal/app"
	"github.com/spf13/cobra"
	"os"
)

const (
	defaultIndexPath = ".sagasu-index.sqlite"
	defaultLimit     = 20
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "sagasu",
		Short: "Local full-text search tool for devs",
	}

	var indexPath string

	rootCmd.PersistentFlags().StringVar(
		&indexPath,
		"index-path",
		defaultIndexPath,
		"path to sqlite index file",
	)

	indexCmd := &cobra.Command{
		Use:   "index [dirs...]",
		Short: "Build indices from directories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := app.RunIndex(args, indexPath)
			if err != nil {
				return err
			}
			return nil
		},
	}

	var extFilters []string
	var limit int
	var jsonOutput bool
	var countOnly bool

	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search indexed content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.RunSearch(args[0], indexPath, app.SearchOptions{
				ExtFilters: extFilters,
				Limit:      limit,
				JSON:       jsonOutput,
				Count:      countOnly,
			})
		},
	}

	searchCmd.Flags().StringSliceVar(&extFilters, "ext", nil, "filter by file extension")
	searchCmd.Flags().IntVar(&limit, "limit", defaultLimit, "maximum number of results")
	searchCmd.Flags().BoolVar(&jsonOutput, "json", false, "output search results as JSON")
	searchCmd.Flags().BoolVar(&countOnly, "count", false, "output only the number of matches")

	rootCmd.AddCommand(indexCmd, searchCmd)

	if err := rootCmd.Execute(); err != nil {
		_, err = fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
