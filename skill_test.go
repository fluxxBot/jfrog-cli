package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreTests "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli/utils/tests"
	rtutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var skillTestRepoName string

func initSkillTest(t *testing.T) {
	if !*tests.TestSkill {
		t.Skip("Skipping Skill test. To run Skill tests add the '-test.skill=true' option.")
	}
	if artifactoryCli == nil {
		initArtifactoryCli()
	}
	createJfrogHomeConfig(t, true)
}

func cleanSkillTest(t *testing.T) {
	clientTestUtils.UnSetEnvAndAssert(t, coreutils.HomeDir)
	tests.CleanFileSystem()
}

func InitSkillTests() {
	initArtifactoryCli()
	skillTestRepoName = "cli-skill-test-" + strconv.FormatInt(time.Now().Unix(), 10)
	createSkillGenericRepo(skillTestRepoName)
	uploadSkillSamplePack(skillTestRepoName)
	uploadSkillExtraPack(skillTestRepoName)
}

func CleanSkillTests() {
	if skillTestRepoName != "" && artifactoryCli != nil {
		_ = artifactoryCli.Exec("repo-delete", skillTestRepoName, "--quiet")
	}
}

func createSkillGenericRepo(repoName string) {
	repoConfig := `{"rclass":"local","packageType":"generic"}`
	rtutils.AddHeader("Content-Type", "application/json", &artHttpDetails.Headers)
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	resp, body, err := client.SendPut(serverDetails.ArtifactoryUrl+"api/repositories/"+repoName, []byte(repoConfig), artHttpDetails, "")
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK, http.StatusCreated); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	log.Info("Created skill test repository:", repoName)
}

func uploadSkillSamplePack(repoName string) {
	samplePackPath := tests.GetTestResourcesPath() + "skill/sample-pack/(.*)"
	targetPath := repoName + "/jfrog/sample-pack/1.0.0/{1}"
	err := artifactoryCli.Exec("upload", samplePackPath, targetPath, "--regexp", "--flat=false")
	if err != nil {
		log.Error("Failed to upload skill sample pack:", err.Error())
		os.Exit(1)
	}
	log.Info("Uploaded skill sample pack to", repoName)
}

func uploadSkillExtraPack(repoName string) {
	extraPackPath := tests.GetTestResourcesPath() + "skill/extra-pack/(.*)"
	targetPath := repoName + "/jfrog/extra-pack/1.0.0/{1}"
	err := artifactoryCli.Exec("upload", extraPackPath, targetPath, "--regexp", "--flat=false")
	if err != nil {
		log.Error("Failed to upload skill extra pack:", err.Error())
		os.Exit(1)
	}
	log.Info("Uploaded skill extra pack to", repoName)
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NoError(t, err, "Expected file to exist: %s", path)
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "Expected file to not exist: %s", path)
}

func TestSkillInstallFull(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "jfrog/sample-pack@1.0.0", "--repo="+skillTestRepoName)
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "error-handling.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "debugging", "trace-issue", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "review", "code-review", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "lint-code.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "run-checks.md"))
	assertFileExists(t, filepath.Join(tempDir, "AGENTS.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "manifest.yaml"))
}

func TestSkillInstallRulesOnly(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "jfrog/sample-pack@1.0.0", "--repo="+skillTestRepoName, "--rules-only")
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "error-handling.mdc"))

	assertFileNotExists(t, filepath.Join(tempDir, ".cursor", "skills"))
	assertFileNotExists(t, filepath.Join(tempDir, ".cursor", "commands"))
	assertFileNotExists(t, filepath.Join(tempDir, "AGENTS.md"))
}

func TestSkillInstallRulesOnlyWithCategory(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "jfrog/sample-pack@1.0.0", "--repo="+skillTestRepoName, "--rules-only", "--category=coding")
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "error-handling.mdc"))

	assertFileNotExists(t, filepath.Join(tempDir, ".cursor", "skills"))
	assertFileNotExists(t, filepath.Join(tempDir, ".cursor", "commands"))
}

func TestSkillInstallNoAgents(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "jfrog/sample-pack@1.0.0", "--repo="+skillTestRepoName, "--no-agents")
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "debugging", "trace-issue", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "lint-code.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "manifest.yaml"))

	assertFileNotExists(t, filepath.Join(tempDir, "AGENTS.md"))
}

func TestSkillInstallLatestVersion(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "jfrog/sample-pack", "--repo="+skillTestRepoName)
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "debugging", "trace-issue", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "lint-code.md"))
	assertFileExists(t, filepath.Join(tempDir, "AGENTS.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "manifest.yaml"))
}

func TestSkillInstallInvalidRef(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")

	err := jfrogCli.Exec("skill", "install", "badref", "--repo="+skillTestRepoName)
	assert.Error(t, err, "Invalid ref should produce an error")
}

func TestSkillHelp(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "--help")
	assert.NoError(t, err, "Help command should not return error")
}

// --- Publish tests ---

func TestSkillPublishFull(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	samplePackDir := tests.GetTestResourcesPath() + "skill/sample-pack"
	cursorDir := filepath.Join(samplePackDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "publish", "testns/pub-pack@2.0.0", "--repo="+skillTestRepoName, "--skill-path="+cursorDir)
	require.NoError(t, err)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	err = jfrogCli.Exec("skill", "install", "testns/pub-pack@2.0.0", "--repo="+skillTestRepoName)
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "error-handling.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "debugging", "trace-issue", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "review", "code-review", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "lint-code.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "run-checks.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "manifest.yaml"))
}

func TestSkillPublishAutoManifest(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempSrc := t.TempDir()
	cursorDir := filepath.Join(tempSrc, ".cursor")
	rulesDir := filepath.Join(cursorDir, "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "test-rule.mdc"), []byte("# test rule"), 0644))

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "publish", "testns/auto-manifest@1.0.0", "--repo="+skillTestRepoName, "--skill-path="+cursorDir)
	require.NoError(t, err)

	assertFileExists(t, filepath.Join(cursorDir, "manifest.yaml"))

	content, err := os.ReadFile(filepath.Join(cursorDir, "manifest.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: auto-manifest")
	assert.Contains(t, string(content), "version: 1.0.0")
	assert.Contains(t, string(content), "rules: 1")

	tempInstall := t.TempDir()
	prevDir := changeWD(t, tempInstall)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	err = jfrogCli.Exec("skill", "install", "testns/auto-manifest@1.0.0", "--repo="+skillTestRepoName)
	require.NoError(t, err)
	assertFileExists(t, filepath.Join(tempInstall, ".cursor", "rules", "test-rule.mdc"))
}

func TestSkillPublishMissingVersion(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "publish", "testns/no-version", "--repo="+skillTestRepoName)
	assert.Error(t, err, "Publish without version should fail")
}

func TestSkillPublishEmptyDir(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempSrc := t.TempDir()
	emptyDir := filepath.Join(tempSrc, ".cursor")
	require.NoError(t, os.MkdirAll(emptyDir, 0755))

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "publish", "testns/empty-pack@1.0.0", "--repo="+skillTestRepoName, "--skill-path="+emptyDir)
	assert.Error(t, err, "Publish from empty directory should fail")
}

// --- Manifest-based install tests ---

func TestSkillInstallFromManifest(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	cursorDir := filepath.Join(tempDir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))

	manifestContent := `dependencies:
  packages:
    - ref: jfrog/sample-pack@1.0.0
  rules:
    - ref: jfrog/extra-pack@1.0.0
  skills:
    - ref: jfrog/extra-pack@1.0.0
  commands:
    - ref: jfrog/extra-pack@1.0.0
`
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "manifest.yaml"), []byte(manifestContent), 0644))

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "--repo="+skillTestRepoName)
	require.NoError(t, err)

	// From sample-pack (full package install)
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "coding", "style-guide.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "debugging", "trace-issue", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "lint-code.md"))

	// From extra-pack (rules-only, skills-only, commands-only)
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "security-check.mdc"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "skills", "analysis", "perf-check", "SKILL.md"))
	assertFileExists(t, filepath.Join(tempDir, ".cursor", "commands", "security-scan.md"))
}

func TestSkillInstallFromManifestMissing(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "--repo="+skillTestRepoName)
	assert.Error(t, err, "Should fail when no manifest.yaml exists")
}

func TestSkillInstallFromManifestEmptyDeps(t *testing.T) {
	initSkillTest(t)
	defer cleanSkillTest(t)

	tempDir := t.TempDir()
	prevDir := changeWD(t, tempDir)
	defer clientTestUtils.ChangeDirAndAssert(t, prevDir)

	cursorDir := filepath.Join(tempDir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))

	manifestContent := `name: empty-project
dependencies: {}
`
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "manifest.yaml"), []byte(manifestContent), 0644))

	jfrogCli := coreTests.NewJfrogCli(execMain, "jfrog", "")
	err := jfrogCli.Exec("skill", "install", "--repo="+skillTestRepoName)
	assert.NoError(t, err, "Empty deps should succeed with no-op")
}
