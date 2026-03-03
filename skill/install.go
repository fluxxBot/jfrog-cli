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
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	servicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

type skillRef struct {
	Namespace string
	Pack      string
	Version   string
}

type localManifest struct {
	Dependencies manifestDependencies `yaml:"dependencies"`
}

type manifestDependencies struct {
	Packages []depEntry `yaml:"packages,omitempty"`
	Rules    []depEntry `yaml:"rules,omitempty"`
	Skills   []depEntry `yaml:"skills,omitempty"`
	Commands []depEntry `yaml:"commands,omitempty"`
}

type depEntry struct {
	Ref      string `yaml:"ref"`
	Category string `yaml:"category,omitempty"`
}

func installCmd(c *cli.Context) error {
	repoName := c.String("repo")
	if repoName == "" {
		return fmt.Errorf("the --repo flag is mandatory")
	}

	serverDetails, err := resolveServerDetails(c)
	if err != nil {
		return err
	}

	if c.NArg() == 0 {
		return installFromManifest(repoName, serverDetails)
	}

	if c.NArg() != 1 {
		return cliutils.WrongNumberOfArgumentsHandler(c)
	}

	ref, err := parseSkillRef(c.Args().Get(0))
	if err != nil {
		return err
	}

	opts := installOpts{
		rulesOnly:    c.Bool("rules-only"),
		skillsOnly:   c.Bool("skills-only"),
		commandsOnly: c.Bool("commands-only"),
		category:     c.String("category"),
		noAgents:     c.Bool("no-agents"),
	}

	return installSinglePack(ref, repoName, opts, serverDetails)
}

type installOpts struct {
	rulesOnly    bool
	skillsOnly   bool
	commandsOnly bool
	category     string
	noAgents     bool
	skipClean    bool
}

func installSinglePack(ref skillRef, repoName string, opts installOpts, serverDetails *config.ServerDetails) error {
	servicesManager, err := createArtifactoryServiceManager(serverDetails)
	if err != nil {
		return err
	}

	if ref.Version == "" {
		resolved, resolveErr := resolveLatestVersion(servicesManager, repoName, ref)
		if resolveErr != nil {
			return resolveErr
		}
		ref.Version = resolved
		log.Info(fmt.Sprintf("Resolved latest version: %s", ref.Version))
	}

	basePath := fmt.Sprintf("%s/%s/%s/%s", repoName, ref.Namespace, ref.Pack, ref.Version)

	manifestPath := basePath + "/manifest.yaml"
	exists, err := pathExistsInRepo(servicesManager, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to check manifest: %w", err)
	}
	if !exists {
		return fmt.Errorf("skill pack not found: %s/%s@%s (manifest.yaml not found at %s)", ref.Namespace, ref.Pack, ref.Version, manifestPath)
	}

	noFilterSet := !opts.rulesOnly && !opts.skillsOnly && !opts.commandsOnly
	contentTypes := buildContentTypeList(noFilterSet, opts.rulesOnly, opts.skillsOnly, opts.commandsOnly)

	var totalInstalled int
	for _, ct := range contentTypes {
		srcPattern := basePath + "/" + ct + "/"
		if opts.category != "" {
			srcPattern += opts.category + "/(*)"
		} else {
			srcPattern += "(*)"
		}

		localDir := filepath.Join(".cursor", ct)
		if opts.category != "" {
			localDir = filepath.Join(localDir, opts.category)
		}
		targetPattern := localDir + "/{1}"

		if !opts.skipClean {
			if err := os.RemoveAll(localDir); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to clean %s: %w", localDir, err)
			}
		}
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", localDir, err)
		}

		count, dlErr := runDownloadCommand(serverDetails, srcPattern, targetPattern, false)
		if dlErr != nil {
			log.Warn(fmt.Sprintf("No %s found or download error: %v", ct, dlErr))
			continue
		}
		totalInstalled += count
		log.Info(fmt.Sprintf("Installed %d %s file(s)", count, ct))
	}

	if !opts.noAgents && noFilterSet {
		agentsPath := basePath + "/AGENTS.md"
		agentExists, agentErr := pathExistsInRepo(servicesManager, agentsPath)
		if agentErr == nil && agentExists {
			_, dlErr := runDownloadCommand(serverDetails, agentsPath, "AGENTS.md", true)
			if dlErr != nil {
				log.Warn(fmt.Sprintf("Failed to download AGENTS.md: %v", dlErr))
			} else {
				log.Info("Installed AGENTS.md")
			}
		}
	}

	if !opts.skipClean {
		if err := upsertLocalManifestDep(ref, opts); err != nil {
			log.Warn(fmt.Sprintf("Failed to update local manifest dependencies: %v", err))
		}
	}

	log.Info(fmt.Sprintf("Successfully installed %d file(s) from %s/%s@%s", totalInstalled, ref.Namespace, ref.Pack, ref.Version))
	return nil
}

func installFromManifest(repoName string, serverDetails *config.ServerDetails) error {
	manifestFile := filepath.Join(".cursor", "manifest.yaml")
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		return fmt.Errorf("no manifest.yaml found at %s: %w", manifestFile, err)
	}

	var m localManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to parse %s: %w", manifestFile, err)
	}

	deps := m.Dependencies
	totalEntries := len(deps.Packages) + len(deps.Rules) + len(deps.Skills) + len(deps.Commands)
	if totalEntries == 0 {
		log.Info("No dependencies found in manifest.yaml")
		return nil
	}

	log.Info(fmt.Sprintf("Installing %d dependency(ies) from manifest.yaml", totalEntries))

	for _, entry := range deps.Packages {
		ref, err := parseSkillRef(entry.Ref)
		if err != nil {
			return fmt.Errorf("invalid package dependency %q: %w", entry.Ref, err)
		}
		log.Info(fmt.Sprintf("Installing package: %s", entry.Ref))
		if err := installSinglePack(ref, repoName, installOpts{skipClean: true}, serverDetails); err != nil {
			return fmt.Errorf("failed to install package %s: %w", entry.Ref, err)
		}
	}

	for _, entry := range deps.Rules {
		ref, err := parseSkillRef(entry.Ref)
		if err != nil {
			return fmt.Errorf("invalid rules dependency %q: %w", entry.Ref, err)
		}
		log.Info(fmt.Sprintf("Installing rules from: %s", entry.Ref))
		opts := installOpts{rulesOnly: true, category: entry.Category, noAgents: true, skipClean: true}
		if err := installSinglePack(ref, repoName, opts, serverDetails); err != nil {
			return fmt.Errorf("failed to install rules from %s: %w", entry.Ref, err)
		}
	}

	for _, entry := range deps.Skills {
		ref, err := parseSkillRef(entry.Ref)
		if err != nil {
			return fmt.Errorf("invalid skills dependency %q: %w", entry.Ref, err)
		}
		log.Info(fmt.Sprintf("Installing skills from: %s", entry.Ref))
		opts := installOpts{skillsOnly: true, noAgents: true, skipClean: true}
		if err := installSinglePack(ref, repoName, opts, serverDetails); err != nil {
			return fmt.Errorf("failed to install skills from %s: %w", entry.Ref, err)
		}
	}

	for _, entry := range deps.Commands {
		ref, err := parseSkillRef(entry.Ref)
		if err != nil {
			return fmt.Errorf("invalid commands dependency %q: %w", entry.Ref, err)
		}
		log.Info(fmt.Sprintf("Installing commands from: %s", entry.Ref))
		opts := installOpts{commandsOnly: true, noAgents: true, skipClean: true}
		if err := installSinglePack(ref, repoName, opts, serverDetails); err != nil {
			return fmt.Errorf("failed to install commands from %s: %w", entry.Ref, err)
		}
	}

	log.Info("All dependencies installed successfully")
	return nil
}

func parseSkillRef(arg string) (skillRef, error) {
	ref := skillRef{}

	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ref, fmt.Errorf("invalid skill reference: %q. Expected format: <namespace>/<pack>@<version>", arg)
	}
	ref.Namespace = parts[0]

	packAndVersion := parts[1]
	if atIdx := strings.Index(packAndVersion, "@"); atIdx != -1 {
		ref.Pack = packAndVersion[:atIdx]
		ref.Version = packAndVersion[atIdx+1:]
		if ref.Pack == "" || ref.Version == "" {
			return ref, fmt.Errorf("invalid skill reference: %q. Pack name and version cannot be empty", arg)
		}
	} else {
		ref.Pack = packAndVersion
	}

	return ref, nil
}

func resolveServerDetails(c *cli.Context) (*config.ServerDetails, error) {
	serverID := c.String("server-id")
	return config.GetSpecificConfig(serverID, true, true)
}

func createArtifactoryServiceManager(serverDetails *config.ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientconfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func pathExistsInRepo(sm artifactory.ArtifactoryServicesManager, path string) (bool, error) {
	searchParams := services.NewSearchParams()
	searchParams.Pattern = path
	searchParams.Recursive = false

	reader, err := sm.SearchFiles(searchParams)
	if err != nil {
		return false, err
	}
	defer func() {
		if reader != nil {
			_ = reader.Close()
		}
	}()
	if reader == nil {
		return false, nil
	}
	length, err := reader.Length()
	if err != nil {
		return false, err
	}
	return length > 0, nil
}

func resolveLatestVersion(sm artifactory.ArtifactoryServicesManager, repoName string, ref skillRef) (string, error) {
	searchParams := services.NewSearchParams()
	searchParams.Pattern = fmt.Sprintf("%s/%s/%s/*/manifest.yaml", repoName, ref.Namespace, ref.Pack)
	searchParams.Recursive = true

	reader, err := sm.SearchFiles(searchParams)
	if err != nil {
		return "", fmt.Errorf("failed to list versions: %w", err)
	}
	defer func() {
		if reader != nil {
			_ = reader.Close()
		}
	}()

	var latestVersion string
	for resultItem := new(servicesutils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(servicesutils.ResultItem) {
		path := resultItem.Path
		pathParts := strings.Split(path, "/")
		if len(pathParts) >= 3 {
			version := pathParts[len(pathParts)-1]
			if latestVersion == "" || version > latestVersion {
				latestVersion = version
			}
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no versions found for %s/%s in repo %s", ref.Namespace, ref.Pack, repoName)
	}
	return latestVersion, nil
}

func runDownloadCommand(serverDetails *config.ServerDetails, pattern, target string, flat bool) (int, error) {
	downloadSpec := spec.NewBuilder().
		Pattern(pattern).
		Target(target).
		Flat(flat).
		Recursive(true).
		BuildSpec()

	downloadCmd := generic.NewDownloadCommand()
	downloadCmd.SetConfiguration(createSkillDownloadConfig()).
		SetServerDetails(serverDetails).
		SetSpec(downloadSpec)

	if err := downloadCmd.Run(); err != nil {
		return 0, err
	}
	result := downloadCmd.Result()
	return result.SuccessCount(), nil
}

func createSkillDownloadConfig() *rtutils.DownloadConfiguration {
	cfg := new(rtutils.DownloadConfiguration)
	cfg.Threads = commonCliUtils.Threads
	return cfg
}

func buildContentTypeList(all, rulesOnly, skillsOnly, commandsOnly bool) []string {
	if all {
		return []string{"rules", "skills", "commands"}
	}
	var types []string
	if rulesOnly {
		types = append(types, "rules")
	}
	if skillsOnly {
		types = append(types, "skills")
	}
	if commandsOnly {
		types = append(types, "commands")
	}
	return types
}

func upsertLocalManifestDep(ref skillRef, opts installOpts) error {
	manifestPath := filepath.Join(".cursor", "manifest.yaml")

	var m localManifest
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		_ = yaml.Unmarshal(data, &m)
	}

	refStr := fmt.Sprintf("%s/%s@%s", ref.Namespace, ref.Pack, ref.Version)
	entry := depEntry{Ref: refStr, Category: opts.category}
	packKey := fmt.Sprintf("%s/%s", ref.Namespace, ref.Pack)

	noFilterSet := !opts.rulesOnly && !opts.skillsOnly && !opts.commandsOnly
	if noFilterSet {
		m.Dependencies.Packages = upsertDep(m.Dependencies.Packages, packKey, entry)
	} else {
		if opts.rulesOnly {
			m.Dependencies.Rules = upsertDep(m.Dependencies.Rules, packKey, entry)
		}
		if opts.skillsOnly {
			m.Dependencies.Skills = upsertDep(m.Dependencies.Skills, packKey, entry)
		}
		if opts.commandsOnly {
			m.Dependencies.Commands = upsertDep(m.Dependencies.Commands, packKey, entry)
		}
	}

	if err := os.MkdirAll(".cursor", 0755); err != nil {
		return fmt.Errorf("failed to create .cursor: %w", err)
	}

	out, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return os.WriteFile(manifestPath, out, 0644)
}

func upsertDep(deps []depEntry, packKey string, entry depEntry) []depEntry {
	for i, d := range deps {
		existingKey := d.Ref
		if atIdx := strings.Index(existingKey, "@"); atIdx != -1 {
			existingKey = existingKey[:atIdx]
		}
		if existingKey == packKey {
			deps[i] = entry
			return deps
		}
	}
	return append(deps, entry)
}
