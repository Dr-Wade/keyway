package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/keywaysh/cli/internal/analytics"
	"github.com/keywaysh/cli/internal/api"
	"github.com/spf13/cobra"
)

// envCmd is the parent command: `keyway env`
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage vault environments",
}

// envCreateCmd is `keyway env create [name]`
var envCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new environment in the vault",
	Long: `Create a new environment in the vault for the current repository.

In a monorepo, the environment name is automatically prefixed with the current
package path. For example, running from packages/functions creates:
  packages/functions/development

You can also provide a fully qualified name explicitly:
  keyway env create packages/functions/staging`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := EnvCreateOptions{
			EnvFlagSet: cmd.Flags().Changed("env"),
		}
		opts.EnvName, _ = cmd.Flags().GetString("env")
		if len(args) > 0 {
			opts.EnvName = args[0]
		}
		return runEnvCreateWithDeps(opts, defaultDeps)
	},
}

// EnvCreateOptions holds options for the env create command
type EnvCreateOptions struct {
	EnvName    string
	EnvFlagSet bool
}

func init() {
	envCreateCmd.Flags().StringP("env", "e", "", "Environment name to create")
	envCmd.AddCommand(envCreateCmd)
}

func runEnvCreateWithDeps(opts EnvCreateOptions, deps *Dependencies) error {
	deps.UI.Intro("env create")

	// Detect repo
	repo, err := deps.Git.DetectRepo()
	if err != nil {
		deps.UI.Error("Not in a git repository with GitHub remote")
		return err
	}
	deps.UI.Step(fmt.Sprintf("Repository: %s", deps.UI.Value(repo)))

	// Determine monorepo package path for env scoping
	packagePath := deps.Git.GetPackagePath()
	if packagePath != "" {
		deps.UI.Step(fmt.Sprintf("Package: %s", deps.UI.Value(packagePath)))
	}

	// Determine environment name
	envName := opts.EnvName

	if envName == "" {
		if !deps.UI.IsInteractive() {
			deps.UI.Error("Environment name is required (provide as argument or use --env)")
			return fmt.Errorf("environment name required")
		}

		// Suggest standard names not yet in the vault
		token, err := deps.Auth.EnsureLogin()
		if err != nil {
			deps.UI.Error(err.Error())
			return err
		}
		client := deps.APIFactory.NewClient(token)
		ctx := context.Background()

		existing, _ := client.GetVaultEnvironments(ctx, repo)
		existingSet := make(map[string]bool, len(existing))
		for _, e := range existing {
			existingSet[e] = true
		}

		suggestions := []string{}
		for _, name := range []string{"development", "staging", "production", "preview", "test"} {
			qualified := qualifyEnvName(packagePath, name)
			if !existingSet[qualified] {
				suggestions = append(suggestions, qualified)
			}
		}
		if len(suggestions) == 0 {
			suggestions = []string{qualifyEnvName(packagePath, "development")}
		}

		selected, err := deps.UI.Select("Environment to create:", suggestions)
		if err != nil {
			return err
		}
		envName = selected
	} else {
		// Qualify the provided name only if it doesn't already contain a slash
		// (i.e. user hasn't provided a fully-qualified path themselves)
		if packagePath != "" && !strings.Contains(envName, "/") {
			envName = qualifyEnvName(packagePath, envName)
		}
	}

	deps.UI.Step(fmt.Sprintf("Environment: %s", deps.UI.Value(envName)))

	// Ensure login (may have already fetched token above in interactive path)
	token, err := deps.Auth.EnsureLogin()
	if err != nil {
		deps.UI.Error(err.Error())
		return err
	}

	client := deps.APIFactory.NewClient(token)
	ctx := context.Background()

	err = deps.UI.Spin(fmt.Sprintf("Creating environment %s...", envName), func() error {
		return client.CreateEnvironment(ctx, repo, envName)
	})

	if err != nil {
		if apiErr, ok := err.(*api.APIError); ok {
			if apiErr.StatusCode == 409 {
				deps.UI.Warn(fmt.Sprintf("Environment '%s' already exists", envName))
				return nil
			}
			deps.UI.Error(apiErr.Error())
			if apiErr.UpgradeURL != "" {
				deps.UI.Message(fmt.Sprintf("Upgrade: %s", deps.UI.Link(apiErr.UpgradeURL)))
			}
		} else {
			deps.UI.Error(err.Error())
		}
		return err
	}

	analytics.Track("cli_env_create", map[string]interface{}{
		"repoFullName": repo,
		"environment":  envName,
		"packagePath":  packagePath,
	})

	deps.UI.Success(fmt.Sprintf("Created environment: %s", envName))
	deps.UI.Message(deps.UI.Dim(fmt.Sprintf("Push secrets with: %s", deps.UI.Command(fmt.Sprintf("keyway push --env %s", envName)))))
	return nil
}
