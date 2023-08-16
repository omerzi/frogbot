package utils

import (
	"context"
	"fmt"
	"github.com/jfrog/frogbot/commands/utils/outputwriter"
	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/jordan-wright/email"
	"net/smtp"
	"strings"
)

var excludedEmailAddresses = []string{"no-reply", "no_reply", "noreply", "no.reply", "frogbot"}

type SecretsEmailDetails struct {
	gitClient       vcsclient.VcsClient
	gitProvider     vcsutils.VcsProvider
	branch          string
	repoName        string
	repoOwner       string
	detectedSecrets []formats.IacSecretsRow
	pullRequestLink string
	EmailDetails
}

func NewSecretsEmailDetails(gitClient vcsclient.VcsClient, gitProvider vcsutils.VcsProvider,
	repoOwner, repoName, branch, pullRequestLink string,
	detectedSecrets []formats.IacSecretsRow, emailDetails EmailDetails) *SecretsEmailDetails {
	return &SecretsEmailDetails{gitClient: gitClient, gitProvider: gitProvider,
		repoOwner: repoOwner, repoName: repoName, branch: branch, pullRequestLink: pullRequestLink,
		detectedSecrets: detectedSecrets, EmailDetails: emailDetails}
}

func AlertSecretsExposed(secretsDetails *SecretsEmailDetails) (err error) {
	if len(secretsDetails.detectedSecrets) == 0 {
		return
	}
	var relevantEmailReceivers []string
	if relevantEmailReceivers, err = getRelevantEmailReceivers(secretsDetails.gitClient, secretsDetails.repoOwner, secretsDetails.repoName, secretsDetails.branch, secretsDetails.EmailReceivers); err != nil {
		return
	}
	secretsDetails.EmailReceivers = append(secretsDetails.EmailReceivers, relevantEmailReceivers...)
	emailDetails := secretsDetails.EmailDetails
	emailContent := getSecretsEmailContent(secretsDetails.detectedSecrets, secretsDetails.gitProvider, secretsDetails.pullRequestLink)
	sender := fmt.Sprintf("JFrog Frogbot <%s>", emailDetails.SmtpUser)
	subject := outputwriter.FrogbotTitlePrefix + "  Frogbot detected potential secrets"
	return sendEmail(sender, subject, emailContent, emailDetails)
}

func getSecretsEmailContent(secrets []formats.IacSecretsRow, gitProvider vcsutils.VcsProvider, pullRequestLink string) string {
	var tableContent strings.Builder
	for _, secret := range secrets {
		tableContent.WriteString(
			fmt.Sprintf(outputwriter.SecretsEmailTableRow,
				secret.File,
				secret.LineColumn,
				secret.Text))
	}
	pullOrMergeRequest := "pull request"
	if gitProvider == vcsutils.GitLab {
		pullOrMergeRequest = "merge request"
	}

	return fmt.Sprintf(
		outputwriter.SecretsEmailHTMLTemplate,
		outputwriter.SecretsEmailCSS,
		pullRequestLink,
		pullOrMergeRequest,
		tableContent.String(),
	)
}

func sendEmail(sender, subject, content string, emailDetails EmailDetails) error {
	e := prepareEmail(sender, subject, content, emailDetails)
	smtpAuth := smtp.PlainAuth("", emailDetails.SmtpUser, emailDetails.SmtpPassword, emailDetails.SmtpServer)
	return e.Send(strings.Join([]string{emailDetails.SmtpServer, emailDetails.SmtpPort}, ":"), smtpAuth)
}

func prepareEmail(sender, subject, content string, emailDetails EmailDetails) *email.Email {
	e := email.NewEmail()
	e.From = sender
	e.To = emailDetails.EmailReceivers
	e.Subject = subject
	e.HTML = []byte(content)
	return e
}

func getRelevantEmailReceivers(client vcsclient.VcsClient, repoOwner, repoName, branch string, emailReceivers []string) ([]string, error) {
	commits, err := client.GetCommits(context.Background(), repoOwner, repoName, branch)
	if err != nil {
		return nil, err
	}

	return getEmailReceiversFromCommits(commits, emailReceivers)
}

func getEmailReceiversFromCommits(commits []vcsclient.CommitInfo, preConfiguredEmailReceivers []string) ([]string, error) {
	emailReceivers := datastructures.MakeSet[string]()
	for _, commit := range commits {
		if shouldExcludeEmailAddress(commit.AuthorEmail, excludedEmailAddresses) || shouldExcludeEmailAddress(commit.AuthorEmail, preConfiguredEmailReceivers) {
			continue
		}
		emailReceivers.Add(commit.AuthorEmail)
	}

	return emailReceivers.ToSlice(), nil
}

func shouldExcludeEmailAddress(emailAddress string, excludes []string) bool {
	for _, excludedEmailAddress := range excludes {
		if excludedEmailAddress != "" && strings.Contains(emailAddress, excludedEmailAddress) {
			return true
		}
	}
	return false
}
