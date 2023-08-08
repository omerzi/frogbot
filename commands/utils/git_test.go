package utils

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestGitManager_GenerateCommitMessage(t *testing.T) {
	tests := []struct {
		gitManager      GitManager
		impactedPackage string
		fixVersion      VulnerabilityDetails
		expected        string
		description     string
	}{
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{commitMessageTemplate: "<type>: bump ${IMPACTED_PACKAGE}"}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "<type>: bump mquery",
			description:     "Custom prefix",
		},
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{commitMessageTemplate: "<type>[scope]: Upgrade package ${IMPACTED_PACKAGE} to ${FIX_VERSION}"}},
			impactedPackage: "mquery", fixVersion: VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:    "<type>[scope]: Upgrade package mquery to 3.4.5",
			description: "Default template",
		}, {
			gitManager:      GitManager{customTemplates: CustomTemplates{commitMessageTemplate: ""}},
			impactedPackage: "mquery", fixVersion: VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:    "Upgrade mquery to 3.4.5",
			description: "Default template",
		},
	}
	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			commitMessage := test.gitManager.GenerateCommitMessage(test.impactedPackage, test.fixVersion.SuggestedFixedVersion)
			assert.Equal(t, test.expected, commitMessage)
		})
	}
}

func TestGitManager_GenerateFixBranchName(t *testing.T) {
	tests := []struct {
		gitManager      GitManager
		impactedPackage string
		fixVersion      VulnerabilityDetails
		expected        string
		description     string
	}{
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{branchNameTemplate: "[Feature]-${IMPACTED_PACKAGE}-${BRANCH_NAME_HASH}"}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "[Feature]-mquery-41b1f45136b25e3624b15999bd57a476",
			description:     "Custom template",
		},
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{branchNameTemplate: ""}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "frogbot-mquery-41b1f45136b25e3624b15999bd57a476",
			description:     "No template",
		}, {
			gitManager:      GitManager{customTemplates: CustomTemplates{branchNameTemplate: "just-a-branch-${BRANCH_NAME_HASH}"}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "just-a-branch-41b1f45136b25e3624b15999bd57a476",
			description:     "Custom template without inputs",
		},
	}
	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			commitMessage, err := test.gitManager.GenerateFixBranchName("md5Branch", test.impactedPackage, test.fixVersion.SuggestedFixedVersion)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, commitMessage)
		})
	}
}

func TestGitManager_GeneratePullRequestTitle(t *testing.T) {
	tests := []struct {
		gitManager      GitManager
		impactedPackage string
		fixVersion      VulnerabilityDetails
		expected        string
		description     string
	}{
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{pullRequestTitleTemplate: "[CustomPR] update ${IMPACTED_PACKAGE} to ${FIX_VERSION}"}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "[CustomPR] update mquery to 3.4.5",
			description:     "Custom template",
		},
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{pullRequestTitleTemplate: "[CustomPR] update ${IMPACTED_PACKAGE}"}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "[CustomPR] update mquery",
			description:     "Custom template one var",
		},
		{
			gitManager:      GitManager{customTemplates: CustomTemplates{pullRequestTitleTemplate: ""}},
			impactedPackage: "mquery",
			fixVersion:      VulnerabilityDetails{SuggestedFixedVersion: "3.4.5"},
			expected:        "[🐸 Frogbot] Update version of mquery to 3.4.5",
			description:     "No prefix",
		},
	}
	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			titleOutput := test.gitManager.GeneratePullRequestTitle(test.impactedPackage, test.fixVersion.SuggestedFixedVersion)
			assert.Equal(t, test.expected, titleOutput)
		})
	}
}

func TestGitManager_GenerateAggregatedFixBranchName(t *testing.T) {
	tests := []struct {
		gitManager GitManager
		expected   string
		desc       string
	}{
		{
			expected:   "frogbot-update-go-dependencies",
			desc:       "No template",
			gitManager: GitManager{},
		},
		{
			expected:   "[feature]-go",
			desc:       "Custom template hash only",
			gitManager: GitManager{customTemplates: CustomTemplates{branchNameTemplate: "[feature]-${BRANCH_NAME_HASH}"}},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			titleOutput, err := test.gitManager.GenerateAggregatedFixBranchName(coreutils.Go)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, titleOutput)
		})
	}
}

func TestGitManager_GenerateAggregatedCommitMessage(t *testing.T) {
	tests := []struct {
		gitManager GitManager
		expected   string
	}{
		{gitManager: GitManager{}, expected: GetAggregatedPullRequestTitle(coreutils.Pipenv)},
		{gitManager: GitManager{customTemplates: CustomTemplates{commitMessageTemplate: "custom_template"}}, expected: "custom_template"},
	}
	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			commit := test.gitManager.GenerateAggregatedCommitMessage(coreutils.Pipenv)
			assert.Equal(t, commit, test.expected)
		})
	}
}

func TestConvertSSHtoHTTPS(t *testing.T) {
	testsCases := []struct {
		repoName    string
		repoOwner   string
		projectName string
		expected    string
		apiEndpoint string
		vcsProvider vcsutils.VcsProvider
	}{
		{
			repoName:    "npmExample",
			repoOwner:   "repoOwner",
			expected:    "https://github.com/repoOwner/npmExample.git",
			apiEndpoint: "https://github.com",
			vcsProvider: vcsutils.GitHub,
		}, {
			repoName:    "npmExample",
			repoOwner:   "repoOwner",
			expected:    "https://api.github.com/repoOwner/npmExample.git",
			apiEndpoint: "https://api.github.com",
			vcsProvider: vcsutils.GitHub,
		},
		{
			repoName:    "npmProject",
			repoOwner:   "myTest5551218",
			apiEndpoint: "https://gitlab.com",
			expected:    "https://gitlab.com/myTest5551218/npmProject.git",
			vcsProvider: vcsutils.GitLab,
		}, {
			repoName:    "onPremProject",
			repoOwner:   "myTest5551218",
			apiEndpoint: "https://gitlab.example.com",
			expected:    "https://gitlab.example.com/myTest5551218/onPremProject.git",
			vcsProvider: vcsutils.GitLab,
		},
		{
			repoName:    "npmExample",
			projectName: "firstProject",
			repoOwner:   "azureReposOwner",
			apiEndpoint: "https://dev.azure.com/azureReposOwner/",
			expected:    "https://azureReposOwner@dev.azure.com/azureReposOwner/firstProject/_git/npmExample",
			vcsProvider: vcsutils.AzureRepos,
		}, {
			repoName:    "npmExample",
			projectName: "onPremProject",
			repoOwner:   "organization",
			apiEndpoint: "https://your-server-name:port/organization/",
			expected:    "https://organization@your-server-name:port/organization/onPremProject/_git/npmExample",
			vcsProvider: vcsutils.AzureRepos,
		},
		{
			repoName:    "npmExample",
			repoOwner:   "~bitbucketServerOwner", // Bitbucket server private projects owners start with ~ prefix.
			apiEndpoint: "https://git.company.info",
			expected:    "https://git.company.info/scm/~bitbucketServerOwner/npmExample.git",
			vcsProvider: vcsutils.BitbucketServer,
		}, {
			repoName:    "npmExample",
			repoOwner:   "bitbucketServerOwner", // Public on prem repo
			apiEndpoint: "https://git.company.info",
			expected:    "https://git.company.info/scm/bitbucketServerOwner/npmExample.git",
			vcsProvider: vcsutils.BitbucketServer,
		}, {
			repoName:    "notSupported",
			repoOwner:   "cloudOwner",
			expected:    "",
			vcsProvider: vcsutils.BitbucketCloud,
		},
	}
	for _, test := range testsCases {
		t.Run(test.vcsProvider.String(), func(t *testing.T) {
			gm := GitManager{git: &Git{GitProvider: test.vcsProvider, RepoName: test.repoName, RepoOwner: test.repoOwner, VcsInfo: vcsclient.VcsInfo{Project: test.projectName, APIEndpoint: test.apiEndpoint}}}
			remoteUrl, err := gm.generateHTTPSCloneUrl()
			if remoteUrl == "" {
				assert.Equal(t, err.Error(), "unsupported version control provider: Bitbucket Cloud")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, remoteUrl)
			}
		})
	}
}

func TestGitManager_Checkout(t *testing.T) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	var restoreWd func() error
	restoreWd, err = Chdir(tmpDir)
	defer func() {
		assert.NoError(t, restoreWd())
	}()
	gitManager := createFakeDotGit(t, tmpDir)
	// Get the current branch that is set as HEAD
	headRef, err := gitManager.repository.Head()
	assert.NoError(t, err)
	assert.Equal(t, headRef.Name().Short(), "master")
	// Create 'dev' branch and checkout
	err = gitManager.CreateBranchAndCheckout("dev")
	assert.NoError(t, err)
	var currBranch string
	currBranch, err = getCurrentBranch(gitManager.repository)
	assert.NoError(t, err)
	assert.Equal(t, "dev", currBranch)
	// Checkout back to 'master'
	assert.NoError(t, gitManager.Checkout("master"))
	currBranch, err = getCurrentBranch(gitManager.repository)
	assert.NoError(t, err)
	assert.Equal(t, "master", currBranch)
}

func createFakeDotGit(t *testing.T, testPath string) *GitManager {
	// Initialize a new in-memory repository
	repo, err := git.PlainInit(testPath, false)
	assert.NoError(t, err)
	// Create a new file and add it to the worktree
	filename := "README.md"
	content := []byte("# My New Repository\n\nThis is a sample repository created using go-git.")
	err = os.WriteFile(filename, content, 0644)
	assert.NoError(t, err)
	worktree, err := repo.Worktree()
	assert.NoError(t, err)
	_, err = worktree.Add(filename)
	assert.NoError(t, err)
	// Commit the changes to the new main branch
	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Your Name",
			Email: "your@email.com",
		},
	})
	assert.NoError(t, err)
	manager, err := NewGitManager(true, testPath, testPath, "origin", "", "", &Git{})
	assert.NoError(t, err)
	return manager
}
