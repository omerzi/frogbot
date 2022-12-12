package commands

import (
	"bytes"
	"fmt"
	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
	coreconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/frogbot/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	clitool "github.com/urfave/cli/v2"
)

const (
	testMultiDirProjConfigPath = "testdata/config/frogbot-config-multi-dir-test-proj.yml"
	testProjSubdirConfigPath   = "testdata/config/frogbot-config-test-proj-subdir.yml"
	testCleanProjConfigPath    = "testdata/config/frogbot-config-clean-test-proj.yml"
	testProjConfigPath         = "testdata/config/frogbot-config-test-proj.yml"
)

func TestCreateXrayScanParams(t *testing.T) {
	// Project
	params := createXrayScanParams(nil, "")
	assert.Empty(t, params.Watches)
	assert.Equal(t, "", params.ProjectKey)
	assert.True(t, params.IncludeVulnerabilities)
	assert.False(t, params.IncludeLicenses)

	// Watches
	params = createXrayScanParams([]string{"watch-1", "watch-2"}, "")
	assert.Equal(t, []string{"watch-1", "watch-2"}, params.Watches)
	assert.Equal(t, "", params.ProjectKey)
	assert.False(t, params.IncludeVulnerabilities)
	assert.False(t, params.IncludeLicenses)

	// Project
	params = createXrayScanParams(nil, "project")
	assert.Empty(t, params.Watches)
	assert.Equal(t, "project", params.ProjectKey)
	assert.False(t, params.IncludeVulnerabilities)
	assert.False(t, params.IncludeLicenses)
}

func TestCreateVulnerabilitiesRows(t *testing.T) {
	// Previous scan with only one violation - XRAY-1
	previousScan := services.ScanResponse{
		Violations: []services.Violation{{
			IssueId:       "XRAY-1",
			Summary:       "summary-1",
			Severity:      "high",
			ViolationType: "security",
			Components:    map[string]services.Component{"component-A": {}, "component-B": {}},
		}},
	}

	// Current scan with 2 violations - XRAY-1 and XRAY-2
	currentScan := services.ScanResponse{
		Violations: []services.Violation{
			{
				IssueId:       "XRAY-1",
				Summary:       "summary-1",
				Severity:      "high",
				ViolationType: "security",
				Components:    map[string]services.Component{"component-A": {}, "component-B": {}},
			},
			{
				IssueId:       "XRAY-2",
				Summary:       "summary-2",
				ViolationType: "security",
				Severity:      "low",
				Components:    map[string]services.Component{"component-C": {}, "component-D": {}},
			},
		},
	}

	// Run createNewIssuesRows and make sure that only the XRAY-2 violation exists in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "XRAY-2", rows[0].IssueId)
	assert.Equal(t, "low", rows[0].Severity)
	assert.Equal(t, "XRAY-2", rows[1].IssueId)
	assert.Equal(t, "low", rows[1].Severity)

	impactedPackageOne := rows[0].ImpactedPackageName
	impactedPackageTwo := rows[1].ImpactedPackageName
	assert.ElementsMatch(t, []string{"component-C", "component-D"}, []string{impactedPackageOne, impactedPackageTwo})
}

func TestCreateVulnerabilitiesRowsCaseNoPrevViolations(t *testing.T) {
	// Previous scan with no violation
	previousScan := services.ScanResponse{
		Violations: []services.Violation{},
	}

	// Current scan with 2 violations - XRAY-1 and XRAY-2
	currentScan := services.ScanResponse{
		Violations: []services.Violation{
			{
				IssueId:       "XRAY-1",
				Summary:       "summary-1",
				Severity:      "high",
				ViolationType: "security",
				Components:    map[string]services.Component{"component-A": {}},
			},
			{
				IssueId:       "XRAY-2",
				Summary:       "summary-2",
				ViolationType: "security",
				Severity:      "low",
				Components:    map[string]services.Component{"component-C": {}},
			},
		},
	}

	// Run createNewIssuesRows and expect both XRAY-1 and XRAY-2 violation in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "XRAY-1", rows[0].IssueId)
	assert.Equal(t, "high", rows[0].Severity)
	assert.Equal(t, "component-A", rows[0].ImpactedPackageName)
	assert.Equal(t, "XRAY-2", rows[1].IssueId)
	assert.Equal(t, "low", rows[1].Severity)
	assert.Equal(t, "component-C", rows[1].ImpactedPackageName)
}

func TestGetNewViolationsCaseNoNewViolations(t *testing.T) {
	// Previous scan with 2 violations - XRAY-1 and XRAY-2
	previousScan := services.ScanResponse{
		Violations: []services.Violation{
			{
				IssueId:       "XRAY-1",
				Summary:       "summary-1",
				Severity:      "high",
				ViolationType: "security",
				Components:    map[string]services.Component{"component-A": {}},
			},
			{
				IssueId:       "XRAY-2",
				Summary:       "summary-2",
				ViolationType: "security",
				Severity:      "low",
				Components:    map[string]services.Component{"component-C": {}},
			},
		},
	}

	// Current scan with no violation
	currentScan := services.ScanResponse{
		Violations: []services.Violation{},
	}

	// Run createNewIssuesRows and expect no violations in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 0)
}

func TestGetAllVulnerabilities(t *testing.T) {
	// Current scan with 2 vulnerabilities - XRAY-1 and XRAY-2
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId:    "XRAY-1",
				Summary:    "summary-1",
				Severity:   "high",
				Components: map[string]services.Component{"component-A": {}, "component-B": {}},
			},
			{
				IssueId:    "XRAY-2",
				Summary:    "summary-2",
				Severity:   "low",
				Components: map[string]services.Component{"component-C": {}, "component-D": {}},
			},
		},
	}

	// Run createAllIssuesRows and make sure that XRAY-1 and XRAY-2 vulnerabilities exists in the results
	rows, err := createAllIssuesRows([]services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 4)
	assert.Equal(t, "XRAY-1", rows[0].IssueId)
	assert.Equal(t, "high", rows[0].Severity)
	assert.Equal(t, "XRAY-1", rows[1].IssueId)
	assert.Equal(t, "high", rows[1].Severity)
	assert.Equal(t, "XRAY-2", rows[2].IssueId)
	assert.Equal(t, "low", rows[2].Severity)
	assert.Equal(t, "XRAY-2", rows[3].IssueId)
	assert.Equal(t, "low", rows[3].Severity)

	impactedPackageOne := rows[0].ImpactedPackageName
	impactedPackageTwo := rows[1].ImpactedPackageName
	assert.ElementsMatch(t, []string{"component-A", "component-B"}, []string{impactedPackageOne, impactedPackageTwo})
	impactedPackageThree := rows[2].ImpactedPackageName
	impactedPackageFour := rows[3].ImpactedPackageName
	assert.ElementsMatch(t, []string{"component-C", "component-D"}, []string{impactedPackageThree, impactedPackageFour})
}

func TestGetNewVulnerabilities(t *testing.T) {
	// Previous scan with only one vulnerability - XRAY-1
	previousScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{{
			IssueId:    "XRAY-1",
			Summary:    "summary-1",
			Severity:   "high",
			Components: map[string]services.Component{"component-A": {}, "component-B": {}},
		}},
	}

	// Current scan with 2 vulnerabilities - XRAY-1 and XRAY-2
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId:    "XRAY-1",
				Summary:    "summary-1",
				Severity:   "high",
				Components: map[string]services.Component{"component-A": {}, "component-B": {}},
			},
			{
				IssueId:    "XRAY-2",
				Summary:    "summary-2",
				Severity:   "low",
				Components: map[string]services.Component{"component-C": {}, "component-D": {}},
			},
		},
	}

	// Run createNewIssuesRows and make sure that only the XRAY-2 vulnerability exists in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "XRAY-2", rows[0].IssueId)
	assert.Equal(t, "low", rows[0].Severity)
	assert.Equal(t, "XRAY-2", rows[1].IssueId)
	assert.Equal(t, "low", rows[1].Severity)

	impactedPackageOne := rows[0].ImpactedPackageName
	impactedPackageTwo := rows[1].ImpactedPackageName
	assert.ElementsMatch(t, []string{"component-C", "component-D"}, []string{impactedPackageOne, impactedPackageTwo})
}

func TestGetNewVulnerabilitiesCaseNoPrevVulnerabilities(t *testing.T) {
	// Previous scan with no vulnerabilities
	previousScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{},
	}

	// Current scan with 2 vulnerabilities - XRAY-1 and XRAY-2
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId:    "XRAY-1",
				Summary:    "summary-1",
				Severity:   "high",
				Components: map[string]services.Component{"component-A": {}},
			},
			{
				IssueId:    "XRAY-2",
				Summary:    "summary-2",
				Severity:   "low",
				Components: map[string]services.Component{"component-B": {}},
			},
		},
	}

	// Run createNewIssuesRows and expect both XRAY-1 and XRAY-2 vulnerability in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "XRAY-1", rows[0].IssueId)
	assert.Equal(t, "high", rows[0].Severity)
	assert.Equal(t, "component-A", rows[0].ImpactedPackageName)
	assert.Equal(t, "XRAY-2", rows[1].IssueId)
	assert.Equal(t, "low", rows[1].Severity)
	assert.Equal(t, "component-B", rows[1].ImpactedPackageName)
}

func TestGetNewVulnerabilitiesCaseNoNewVulnerabilities(t *testing.T) {
	// Previous scan with 2 vulnerabilities - XRAY-1 and XRAY-2
	previousScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId:    "XRAY-1",
				Summary:    "summary-1",
				Severity:   "high",
				Components: map[string]services.Component{"component-A": {}},
			},
			{
				IssueId:    "XRAY-2",
				Summary:    "summary-2",
				Severity:   "low",
				Components: map[string]services.Component{"component-B": {}},
			},
		},
	}

	// Current scan with no vulnerabilities
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{},
	}

	// Run createNewIssuesRows and expect no vulnerability in the results
	rows, err := createNewIssuesRows([]services.ScanResponse{previousScan}, []services.ScanResponse{currentScan})
	assert.NoError(t, err)
	assert.Len(t, rows, 0)
}

func TestCreatePullRequestMessageNoVulnerabilities(t *testing.T) {
	vulnerabilities := []formats.VulnerabilityOrViolationRow{}
	message := createPullRequestMessage(vulnerabilities, utils.GetBanner, utils.GetSeverityTag)

	expectedMessageByte, err := os.ReadFile(filepath.Join("testdata", "messages", "novulnerabilities.md"))
	assert.NoError(t, err)
	expectedMessage := strings.ReplaceAll(string(expectedMessageByte), "\r\n", "\n")
	assert.Equal(t, expectedMessage, message)
}

func TestCreatePullRequestMessage(t *testing.T) {
	vulnerabilities := []formats.VulnerabilityOrViolationRow{
		{
			Severity:               "High",
			ImpactedPackageName:    "github.com/nats-io/nats-streaming-server",
			ImpactedPackageVersion: "v0.21.0",
			FixedVersions:          []string{"[0.24.1]"},
			Components: []formats.ComponentRow{
				{
					Name:    "github.com/nats-io/nats-streaming-server",
					Version: "v0.21.0",
				},
			},
			Cves: []formats.CveRow{{Id: "CVE-2022-24450"}},
		},
		{
			Severity:               "High",
			ImpactedPackageName:    "github.com/mholt/archiver/v3",
			ImpactedPackageVersion: "v3.5.1",
			Components: []formats.ComponentRow{
				{
					Name:    "github.com/mholt/archiver/v3",
					Version: "v3.5.1",
				},
			},
			Cves: []formats.CveRow{},
		},
		{
			Severity:               "Medium",
			ImpactedPackageName:    "github.com/nats-io/nats-streaming-server",
			ImpactedPackageVersion: "v0.21.0",
			FixedVersions:          []string{"[0.24.3]"},
			Components: []formats.ComponentRow{
				{
					Name:    "github.com/nats-io/nats-streaming-server",
					Version: "v0.21.0",
				},
			},
			Cves: []formats.CveRow{{Id: "CVE-2022-26652"}},
		},
	}
	message := createPullRequestMessage(vulnerabilities, utils.GetBanner, utils.GetSeverityTag)

	expectedMessageByte, err := os.ReadFile(filepath.Join("testdata", "messages", "dummyvulnerabilities.md"))
	assert.NoError(t, err)
	expectedMessage := strings.ReplaceAll(string(expectedMessageByte), "\r\n", "\n")
	assert.Equal(t, expectedMessage, message)
}

func TestRunInstallIfNeeded(t *testing.T) {
	assert.NoError(t, runInstallIfNeeded(&utils.Project{}, "", true))
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	params := &utils.Project{
		InstallCommandName: "echo",
		InstallCommandArgs: []string{"Hello"},
	}
	assert.NoError(t, runInstallIfNeeded(params, tmpDir, true))

	params = &utils.Project{
		InstallCommandName: "not-existed",
		InstallCommandArgs: []string{"1", "2"},
	}
	assert.NoError(t, runInstallIfNeeded(params, tmpDir, false))

	params = &utils.Project{
		InstallCommandName: "not-existed",
		InstallCommandArgs: []string{"1", "2"},
	}
	assert.Error(t, runInstallIfNeeded(params, tmpDir, true))
}

func TestScanPullRequest(t *testing.T) {
	testScanPullRequest(t, testProjConfigPath, "test-proj", true)
}

func TestScanPullRequestNoFail(t *testing.T) {
	testScanPullRequest(t, testProjConfigPath, "test-proj", false)
}

func TestScanPullRequestSubdir(t *testing.T) {
	testScanPullRequest(t, testProjSubdirConfigPath, "test-proj-subdir", true)
}

func TestScanPullRequestNoIssues(t *testing.T) {
	testScanPullRequest(t, testCleanProjConfigPath, "clean-test-proj", false)
}

func TestScanPullRequestMultiWorkDir(t *testing.T) {
	testScanPullRequest(t, testMultiDirProjConfigPath, "multi-dir-test-proj", true)
}

func TestScanPullRequestMultiWorkDirNoFail(t *testing.T) {
	testScanPullRequest(t, testMultiDirProjConfigPath, "multi-dir-test-proj", false)
}

func testScanPullRequest(t *testing.T, configPath, projectName string, failOnSecurityIssues bool) {
	params, restoreEnv := verifyEnv(t)
	defer restoreEnv()

	// Create mock GitLab server
	server := httptest.NewServer(createGitLabHandler(t, projectName))
	defer server.Close()

	configAggregator, client := prepareConfigAndClient(t, configPath, failOnSecurityIssues, server, params)
	cleanUp := prepareTestEnvironment(t, projectName)
	defer cleanUp()

	// Run "frogbot scan pull request"
	var scanPullRequest ScanPullRequestCmd
	err := scanPullRequest.Run(configAggregator, client)
	if failOnSecurityIssues {
		assert.EqualErrorf(t, err, securityIssueFoundErr, "Error should be: %v, got: %v", securityIssueFoundErr, err)
	} else {
		assert.NoError(t, err)
	}

	// Check env sanitize
	err = utils.SanitizeEnv()
	assert.NoError(t, err)
	utils.AssertSanitizedEnv(t)
}

func prepareConfigAndClient(t *testing.T, configPath string, failOnSecurityIssues bool, server *httptest.Server, serverParams coreconfig.ServerDetails) (utils.FrogbotConfigAggregator, vcsclient.VcsClient) {
	gitParams := utils.Git{
		GitProvider:   vcsutils.GitLab,
		RepoOwner:     "jfrog",
		Token:         "123456",
		ApiEndpoint:   server.URL,
		PullRequestID: 1,
	}

	var configData *utils.FrogbotConfigAggregator
	var err error
	if configPath == "" {
		configData = &utils.FrogbotConfigAggregator{{}}
	} else {
		configData, err = utils.ReadConfig(configPath)
	}
	assert.NoError(t, err)
	var configAggregator utils.FrogbotConfigAggregator
	for _, config := range *configData {
		gitParams.RepoName = config.RepoName
		gitParams.Branches = config.Branches
		config.FailOnSecurityIssues = &failOnSecurityIssues
		config.Git = gitParams
		params := utils.Params{
			Scan:          config.Scan,
			Git:           gitParams,
			JFrogPlatform: config.JFrogPlatform,
		}
		configAggregator = append(configAggregator, utils.FrogbotRepoConfig{
			Server:           serverParams,
			SimplifiedOutput: config.SimplifiedOutput,
			Params:           params,
		})
	}

	client, err := vcsclient.NewClientBuilder(vcsutils.GitLab).ApiEndpoint(server.URL).Token("123456").Build()
	assert.NoError(t, err)
	return configAggregator, client
}

// Prepare test environment for the integration tests
// projectName - the directory name under testdata/scanpullrequests
// Return a cleanup function
func prepareTestEnvironment(t *testing.T, projectName string) func() {
	// Copy project to a temporary directory
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	err = fileutils.CopyDir(filepath.Join("testdata", "scanpullrequest"), tmpDir, true, []string{})
	assert.NoError(t, err)

	restoreDir, err := utils.Chdir(filepath.Join(tmpDir, projectName))
	assert.NoError(t, err)
	return func() {
		assert.NoError(t, restoreDir())
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}
}

func TestScanPullRequestError(t *testing.T) {
	app := clitool.App{Commands: GetCommands()}
	assert.Error(t, app.Run([]string{"frogbot", "spr"}))
}

// Create HTTP handler to mock GitLab server
func createGitLabHandler(t *testing.T, projectName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return 200 on ping
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Return test-proj.tar.gz when using DownloadRepository
		if r.RequestURI == fmt.Sprintf("/api/v4/projects/jfrog%s/repository/archive.tar.gz?sha=master", "%2F"+projectName) {
			w.WriteHeader(http.StatusOK)
			repoFile, err := os.ReadFile(filepath.Join("..", projectName+".tar.gz"))
			assert.NoError(t, err)
			_, err = w.Write(repoFile)
			assert.NoError(t, err)
		}
		// clean-test-proj should not include any vulnerabilities so assertion is not needed.
		if r.RequestURI == fmt.Sprintf("/api/v4/projects/jfrog%s/merge_requests/1/notes", "%2Fclean-test-proj") {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{}"))
			assert.NoError(t, err)
			return
		}

		// Return 200 when using the REST that creates the comment
		if r.RequestURI == fmt.Sprintf("/api/v4/projects/jfrog%s/merge_requests/1/notes", "%2F"+projectName) {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())

			var expectedResponse []byte
			if strings.Contains(projectName, "multi-dir") {
				expectedResponse, err = os.ReadFile(filepath.Join("..", "expectedResponseMultiDir.json"))
			} else if strings.Contains(projectName, "pip") {
				expectedResponse, err = os.ReadFile(filepath.Join("..", "expectedResponsePip.json"))
			} else {
				expectedResponse, err = os.ReadFile(filepath.Join("..", "expectedResponse.json"))
			}
			assert.NoError(t, err)
			assert.JSONEq(t, string(expectedResponse), buf.String())

			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{}"))
			assert.NoError(t, err)
		}
	}
}

// Check connection details with JFrog instance.
// Return a callback method that restores the credentials after the test is done.
func verifyEnv(t *testing.T) (server coreconfig.ServerDetails, restoreFunc func()) {
	url := strings.TrimSuffix(os.Getenv(utils.JFrogUrlEnv), "/")
	username := os.Getenv(utils.JFrogUserEnv)
	password := os.Getenv(utils.JFrogPasswordEnv)
	token := os.Getenv(utils.JFrogTokenEnv)
	if url == "" {
		assert.FailNow(t, fmt.Sprintf("'%s' is not set", utils.JFrogUrlEnv))
	}
	if token == "" && (username == "" || password == "") {
		assert.FailNow(t, fmt.Sprintf("'%s' or '%s' and '%s' are not set", utils.JFrogTokenEnv, utils.JFrogUserEnv, utils.JFrogPasswordEnv))
	}
	server.Url = url
	server.XrayUrl = url + "/xray/"
	server.ArtifactoryUrl = url + "/artifactory/"
	server.User = username
	server.Password = password
	server.AccessToken = token
	restoreFunc = func() {
		utils.SetEnvAndAssert(t, map[string]string{
			utils.JFrogUrlEnv:      url,
			utils.JFrogTokenEnv:    token,
			utils.JFrogUserEnv:     username,
			utils.JFrogPasswordEnv: password,
		})
	}
	return
}

func TestGetFullPathWorkingDirs(t *testing.T) {
	sampleProject := utils.Project{
		WorkingDirs: []string{filepath.Join("a", "b"), filepath.Join("a", "b", "c"), ".", filepath.Join("c", "d", "e", "f")},
	}
	baseWd := "tempDir"
	fullPathWds := getFullPathWorkingDirs(&sampleProject, baseWd)
	expectedWds := []string{filepath.Join("tempDir", "a", "b"), filepath.Join("tempDir", "a", "b", "c"), "tempDir", filepath.Join("tempDir", "c", "d", "e", "f")}
	for _, expectedWd := range expectedWds {
		assert.Contains(t, fullPathWds, expectedWd)
	}
}
