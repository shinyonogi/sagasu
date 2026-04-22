package main

import (
	"fmt"
	"github.com/shinyonogi/sagasu/internal/app"
	"github.com/shinyonogi/sagasu/internal/embedding"
	"github.com/shinyonogi/sagasu/internal/indexpath"
	"github.com/spf13/cobra"
	"os"
)

const (
	defaultLimit      = 20
	defaultConfigPath = ".sagasu.json"
)

func main() {
	rootCmd := buildRootCommand()

	if err := rootCmd.Execute(); err != nil {
		_, err = fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}

func buildRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sagasu",
		Short: "Local full-text search tool for devs",
	}

	var indexPath string
	var rootPath string
	var configPath string
	var semantic bool
	var embeddingProvider string
	var embeddingModel string
	var ollamaURL string

	rootCmd.PersistentFlags().StringVar(
		&indexPath,
		"index-path",
		"",
		"path to sqlite index file (overrides the managed global index path)",
	)
	rootCmd.PersistentFlags().StringVar(
		&rootPath,
		"root",
		"",
		"repository root used to resolve the managed global index path",
	)
	rootCmd.PersistentFlags().StringVar(
		&configPath,
		"config",
		defaultConfigPath,
		"path to sagasu config file",
	)

	var indexJSON bool

	indexCmd := &cobra.Command{
		Use:   "index [dirs...]",
		Short: "Build indices from directories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIndexPath, err := indexpath.ResolveForRoots(indexPath, args)
			if err != nil {
				return err
			}
			err = app.RunIndex(args, resolvedIndexPath, app.IndexOptions{
				ConfigPath:        configPath,
				JSON:              indexJSON,
				EnableSemantic:    semantic,
				EmbeddingProvider: embeddingProvider,
				EmbeddingModel:    embeddingModel,
				OllamaURL:         ollamaURL,
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	indexCmd.Flags().BoolVar(&indexJSON, "json", false, "output index summary as JSON")
	indexCmd.Flags().BoolVar(&semantic, "semantic", false, "generate local embeddings for semantic search")
	indexCmd.Flags().StringVar(&embeddingProvider, "embedding-provider", embedding.DefaultProvider, "embedding provider to use when semantic mode is enabled")
	indexCmd.Flags().StringVar(&embeddingModel, "embedding-model", embedding.DefaultModel, "embedding model to use when semantic mode is enabled")
	indexCmd.Flags().StringVar(&ollamaURL, "ollama-url", embedding.DefaultOllama, "base URL for the local Ollama server")

	var extFilters []string
	var limit int
	var jsonOutput bool
	var countOnly bool
	var contextLines int
	var pathOnly bool
	var filesWithMatches bool
	var statusJSON bool
	var doctorJSON bool
	var semanticWeight float64

	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search indexed content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIndexPath, err := indexpath.ResolveForRoot(indexPath, rootPath)
			if err != nil {
				return err
			}
			return app.RunSearch(args[0], resolvedIndexPath, app.SearchOptions{
				ExtFilters:        extFilters,
				Limit:             limit,
				JSON:              jsonOutput,
				Count:             countOnly,
				Context:           contextLines,
				PathOnly:          pathOnly,
				FilesOnly:         filesWithMatches,
				EnableSemantic:    semantic,
				EmbeddingProvider: embeddingProvider,
				EmbeddingModel:    embeddingModel,
				OllamaURL:         ollamaURL,
				SemanticWeight:    semanticWeight,
			})
		},
	}

	searchCmd.Flags().StringSliceVar(&extFilters, "ext", nil, "filter by file extension")
	searchCmd.Flags().IntVar(&limit, "limit", defaultLimit, "maximum number of results")
	searchCmd.Flags().BoolVar(&jsonOutput, "json", false, "output search results as JSON")
	searchCmd.Flags().BoolVar(&countOnly, "count", false, "output only the number of matches")
	searchCmd.Flags().BoolVar(&pathOnly, "path-only", false, "output match locations as path:line")
	searchCmd.Flags().BoolVar(&filesWithMatches, "files-with-matches", false, "output unique file paths with matches")
	searchCmd.Flags().IntVarP(&contextLines, "context", "C", 0, "show N lines of context around each match")
	searchCmd.Flags().BoolVar(&semantic, "semantic", false, "blend lexical results with local semantic search")
	searchCmd.Flags().StringVar(&embeddingProvider, "embedding-provider", embedding.DefaultProvider, "embedding provider to use when semantic mode is enabled")
	searchCmd.Flags().StringVar(&embeddingModel, "embedding-model", embedding.DefaultModel, "embedding model to use when semantic mode is enabled")
	searchCmd.Flags().StringVar(&ollamaURL, "ollama-url", embedding.DefaultOllama, "base URL for the local Ollama server")
	searchCmd.Flags().Float64Var(&semanticWeight, "semantic-weight", 2.0, "weight applied to semantic scores when semantic mode is enabled")

	statusCmd := &cobra.Command{
		Use:     "status",
		Short:   "Show index metadata and stats",
		Aliases: []string{"info"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIndexPath, err := indexpath.ResolveForRoot(indexPath, rootPath)
			if err != nil {
				return err
			}
			return app.RunStatus(resolvedIndexPath, app.StatusOptions{
				JSON: statusJSON,
			})
		},
	}
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output index stats as JSON")

	rebuildCmd := &cobra.Command{
		Use:   "rebuild [dirs...]",
		Short: "Rebuild the index from scratch",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIndexPath, err := indexpath.ResolveForRoots(indexPath, args)
			if err != nil {
				return err
			}
			return app.RunRebuild(args, resolvedIndexPath, app.IndexOptions{
				ConfigPath:        configPath,
				JSON:              indexJSON,
				EnableSemantic:    semantic,
				EmbeddingProvider: embeddingProvider,
				EmbeddingModel:    embeddingModel,
				OllamaURL:         ollamaURL,
			})
		},
	}
	rebuildCmd.Flags().BoolVar(&indexJSON, "json", false, "output rebuild summary as JSON")
	rebuildCmd.Flags().BoolVar(&semantic, "semantic", false, "regenerate local embeddings for semantic search")
	rebuildCmd.Flags().StringVar(&embeddingProvider, "embedding-provider", embedding.DefaultProvider, "embedding provider to use when semantic mode is enabled")
	rebuildCmd.Flags().StringVar(&embeddingModel, "embedding-model", embedding.DefaultModel, "embedding model to use when semantic mode is enabled")
	rebuildCmd.Flags().StringVar(&ollamaURL, "ollama-url", embedding.DefaultOllama, "base URL for the local Ollama server")

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check index health and stale documents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedIndexPath, err := indexpath.ResolveForRoot(indexPath, rootPath)
			if err != nil {
				return err
			}
			return app.RunDoctor(resolvedIndexPath, app.DoctorOptions{
				JSON: doctorJSON,
			})
		},
	}
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "output doctor results as JSON")

	completionCmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	rootCmd.AddCommand(indexCmd, rebuildCmd, searchCmd, statusCmd, doctorCmd, completionCmd)
	return rootCmd
}
