package skill

import (
	corecommon "github.com/jfrog/jfrog-cli-core/v2/docs/common"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/urfave/cli"
)

func GetCommands() []cli.Command {
	return cliutils.GetSortedCommands(cli.CommandsByName{
		{
			Name:         "install",
			Flags:        cliutils.GetCommandFlags(cliutils.SkillInstall),
			Usage:        "Install AI agent skills, rules, and commands from an Artifactory repository. With no arguments, installs all dependencies from .cursor/manifest.yaml.",
			HelpName:     corecommon.CreateUsage("skill install", "Install AI agent skills, rules, and commands from an Artifactory repository.", []string{"skill install --repo=<repo-name>", "skill install <namespace>/<pack>@<version> --repo=<repo-name> [flags]"}),
			ArgsUsage:    "[<namespace>/<pack-name>@<version>]",
			BashComplete: corecommon.CreateBashCompletionFunc(),
			Action:       installCmd,
		},
		{
			Name:         "publish",
			Flags:        cliutils.GetCommandFlags(cliutils.SkillPublish),
			Usage:        "Publish local AI agent skills, rules, and commands to an Artifactory repository.",
			HelpName:     corecommon.CreateUsage("skill publish", "Publish local AI agent skills, rules, and commands to an Artifactory repository.", []string{"skill publish <namespace>/<pack>@<version> --repo=<repo-name> [flags]"}),
			ArgsUsage:    "<namespace>/<pack-name>@<version>",
			BashComplete: corecommon.CreateBashCompletionFunc(),
			Action:       publishCmd,
		},
	})
}
