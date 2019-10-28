// Copyright 2016-2019, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"
)

type newPolicyArgs struct {
	dir               string
	force             bool
	generateOnly      bool
	interactive       bool
	offline           bool
	templateNameOrURL string
	yes               bool
}

func newPolicyNewCmd() *cobra.Command {
	args := newPolicyArgs{
		interactive: cmdutil.Interactive(),
	}

	cmd := &cobra.Command{
		Use:        "new [template|url]",
		SuggestFor: []string{"init", "create"},
		Short:      "Create a new Pulumi Policy Pack",
		Long: "Create a new Pulumi Policy Pack from a template.\n" +
			"\n" +
			"To create a Policy Pack from a specific template, pass the template name (such as `aws-typescript`\n" +
			"or `azure-python`).  If no template name is provided, a list of suggested templates will be presented\n" +
			"which can be selected interactively.\n" +
			"\n" +
			"Once you're done authoring the Policy Pack, you will need to publish the pack to your organization.\n" +
			"Only organization administrators can publish a Policy Pack.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			if len(cliArgs) > 0 {
				args.templateNameOrURL = cliArgs[0]
			}
			return runNewPolicyPack(args)
		}),
	}

	cmd.PersistentFlags().StringVar(
		&args.dir, "dir", "",
		"The location to place the generated Policy Pack; if not specified, the current directory is used")
	cmd.PersistentFlags().BoolVarP(
		&args.force, "force", "f", false,
		"Forces content to be generated even if it would change existing files")
	cmd.PersistentFlags().BoolVarP(
		&args.generateOnly, "generate-only", "g", false,
		"Generate the Policy Pack only; do not install dependencies")
	cmd.PersistentFlags().BoolVarP(
		&args.offline, "offline", "o", false,
		"Use locally cached templates without making any network requests")
	cmd.PersistentFlags().BoolVarP(
		&args.yes, "yes", "y", false,
		"Skip prompts and proceed with default values")

	return cmd
}

func runNewPolicyPack(args newPolicyArgs) error {
	// TODO - is this required?
	if !args.interactive {
		args.yes = true // auto-approve changes, since we cannot prompt.
	}

	// TODO - is this required?
	// Prepare options.
	opts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: args.interactive,
	}

	// Get the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting the working directory")
	}

	// If dir was specified, ensure it exists and use it as the
	// current working directory.
	if args.dir != "" {
		// Ensure the directory exists.
		if err = os.MkdirAll(args.dir, os.ModePerm); err != nil {
			return errors.Wrap(err, "creating the directory")
		}

		// Change the working directory to the specified directory.
		if err = os.Chdir(args.dir); err != nil {
			return errors.Wrap(err, "changing the working directory")
		}

		// Get the new working directory.
		if cwd, err = os.Getwd(); err != nil {
			return errors.Wrap(err, "getting the working directory")
		}
	}

	// Return an error if the directory isn't empty.
	if !args.force {
		if err = errorIfNotEmptyDirectory(cwd); err != nil {
			return err
		}
	}

	// Retrieve the templates-policy repo.
	repo, err := workspace.RetrieveTemplates(args.templateNameOrURL, args.offline, workspace.TemplateKindPolicyPack)
	if err != nil {
		return err
	}
	defer func() {
		contract.IgnoreError(repo.Delete())
	}()

	// List the templates from the repo.
	templates, err := repo.PolicyTemplates()
	if err != nil {
		return err
	}

	var template workspace.PolicyPackTemplate
	if len(templates) == 0 {
		return errors.New("no templates")
	} else if len(templates) == 1 {
		template = templates[0]
	} else {
		if template, err = choosePolicyPackTemplate(templates, opts); err != nil {
			return err
		}
	}

	// Do a dry run, if we're not forcing files to be overwritten.
	if !args.force {
		if err = template.CopyPolicyTemplateFilesDryRun(cwd); err != nil {
			if os.IsNotExist(err) {
				return errors.Wrapf(err, "template '%s' not found", args.templateNameOrURL)
			}
			return err
		}
	}

	// Actually copy the files.
	if err = template.CopyPolicyPackTemplateFiles(cwd, args.force, "", ""); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrapf(err, "template '%s' not found", args.templateNameOrURL)
		}
		return err
	}

	fmt.Println("Created policy pack!")

	// Install dependencies.
	if !args.generateOnly {
		if err := npmInstallDependencies(); err != nil {
			return err
		}
	}

	fmt.Println(
		opts.Color.Colorize(
			colors.BrightGreen+colors.Bold+"Your new policy pack is ready to go!"+colors.Reset) +
			" " + cmdutil.EmojiOr("✨", ""))
	fmt.Println()

	fmt.Println("Once you're done editting your Policy Pack, run `pulumi policy publish <organization>/<policy_pack_name>` to publish the pack.")
	return nil
}
