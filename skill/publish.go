package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-artifactory/artifactory/commands/generic"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonCliUtils "github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

type manifest struct {
	Name         string               `yaml:"name"`
	Namespace    string               `yaml:"namespace"`
	Version      string               `yaml:"version"`
	Agent        string               `yaml:"agent"`
	Description  string               `yaml:"description,omitempty"`
	Contents     manifestContents     `yaml:"contents"`
	Dependencies manifestDependencies `yaml:"dependencies,omitempty"`
}

type manifestContents struct {
	Skills   int  `yaml:"skills"`
	Rules    int  `yaml:"rules"`
	Commands int  `yaml:"commands"`
	AgentsMd bool `yaml:"agents_md"`
}

func publishCmd(c *cli.Context) error {
	if c.NArg() != 1 {
		return cliutils.WrongNumberOfArgumentsHandler(c)
	}

	repoName := c.String("repo")
	if repoName == "" {
		return fmt.Errorf("the --repo flag is mandatory")
	}

	ref, err := parseSkillRef(c.Args().Get(0))
	if err != nil {
		return err
	}
	if ref.Version == "" {
		return fmt.Errorf("version is mandatory for publish (use <namespace>/<pack>@<version>)")
	}

	cursorDir := c.String("skill-path")
	if cursorDir == "" {
		cursorDir = ".cursor"
	}

	absDir, err := filepath.Abs(cursorDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", cursorDir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("directory not found: %s", absDir)
	}

	projectRoot := filepath.Dir(absDir)
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	hasAgents := fileExists(agentsPath)

	contentCounts := map[string]int{}
	totalFiles := 0
	for _, ct := range []string{"rules", "skills", "commands"} {
		count := countFiles(filepath.Join(absDir, ct))
		contentCounts[ct] = count
		totalFiles += count
	}

	if totalFiles == 0 && !hasAgents {
		return fmt.Errorf("nothing to publish: no rules, skills, commands, or AGENTS.md found in %s", absDir)
	}

	m := manifest{
		Name:      ref.Pack,
		Namespace: ref.Namespace,
		Version:   ref.Version,
		Agent:     "cursor",
		Contents: manifestContents{
			Skills:   contentCounts["skills"],
			Rules:    contentCounts["rules"],
			Commands: contentCounts["commands"],
			AgentsMd: hasAgents,
		},
	}
	manifestBytes, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}
	manifestFile := filepath.Join(absDir, "manifest.yaml")
	if err := os.WriteFile(manifestFile, manifestBytes, 0644); err != nil {
		return fmt.Errorf("failed to write manifest.yaml: %w", err)
	}
	log.Info("Updated manifest.yaml")

	serverDetails, err := resolveServerDetails(c)
	if err != nil {
		return err
	}
	servicesManager, err := createArtifactoryServiceManager(serverDetails)
	if err != nil {
		return err
	}

	basePath := fmt.Sprintf("%s/%s/%s/%s", repoName, ref.Namespace, ref.Pack, ref.Version)

	exists, _ := pathExistsInRepo(servicesManager, basePath+"/manifest.yaml")
	if exists {
		log.Warn(fmt.Sprintf("Version %s already exists in %s, files will be overwritten", ref.Version, repoName))
	}

	srcPattern := filepath.ToSlash(absDir) + "/(.*)"
	targetPattern := basePath + "/{1}"
	result, err := runUploadCommand(serverDetails, srcPattern, targetPattern, true)
	if err != nil {
		return fmt.Errorf("failed to upload content: %w", err)
	}
	uploaded := result

	if hasAgents {
		agentsSrc := filepath.ToSlash(agentsPath)
		agentsTarget := basePath + "/AGENTS.md"
		agentsResult, agentsErr := runUploadCommand(serverDetails, agentsSrc, agentsTarget, false)
		if agentsErr != nil {
			log.Warn(fmt.Sprintf("Failed to upload AGENTS.md: %v", agentsErr))
		} else {
			uploaded += agentsResult
			log.Info("Published AGENTS.md")
		}
	}

	log.Info(fmt.Sprintf("Successfully published %d file(s) to %s/%s@%s", uploaded, ref.Namespace, ref.Pack, ref.Version))
	return nil
}

func runUploadCommand(serverDetails *config.ServerDetails, pattern, target string, regexp bool) (int, error) {
	specBuilder := spec.NewBuilder().
		Pattern(pattern).
		Target(target).
		Recursive(true)
	if regexp {
		specBuilder = specBuilder.Regexp(true)
	}
	uploadSpec := specBuilder.BuildSpec()

	uploadCmd := generic.NewUploadCommand()
	uploadCmd.SetUploadConfiguration(createSkillUploadConfig()).
		SetServerDetails(serverDetails).
		SetSpec(uploadSpec)

	if err := uploadCmd.Run(); err != nil {
		return 0, err
	}
	result := uploadCmd.Result()
	return result.SuccessCount(), nil
}

func createSkillUploadConfig() *rtutils.UploadConfiguration {
	cfg := new(rtutils.UploadConfiguration)
	cfg.Threads = commonCliUtils.Threads
	return cfg
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func countFiles(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			count++
		}
		return nil
	})
	return count
}
