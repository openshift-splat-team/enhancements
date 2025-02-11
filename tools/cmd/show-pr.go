package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift/enhancements/tools/enhancements"
	"github.com/openshift/enhancements/tools/report"
	"github.com/openshift/enhancements/tools/stats"
	"github.com/openshift/enhancements/tools/util"
)

func newShowPRCommand() *cobra.Command {
	var daysBack int

	cmd := &cobra.Command{
		Use:       "show-pr",
		Short:     "Dump details for a pull request",
		ValidArgs: []string{"pull-request-id"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("please specify one valid pull request ID")
			}
			if _, err := strconv.Atoi(args[0]); err != nil {
				return fmt.Errorf("pull request ID %q must be an integer: %w", args[0], err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			initConfig()

			prID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("failed to interpret pull request ID %q as a number: %w",
					args[0], err)
			}

			ghClient := util.NewGithubClient(configSettings.Github.Token)
			ctx := context.Background()
			pr, _, err := ghClient.PullRequests.Get(ctx, orgName, repoName, prID)
			if err != nil {
				return fmt.Errorf("failed to fetch pull request %d: %w", prID, err)
			}

			earliestDate := time.Now().AddDate(0, 0, daysBack*-1)

			query := &util.PullRequestQuery{
				Org:     orgName,
				Repo:    repoName,
				DevMode: false,
				Client:  ghClient,
			}

			// Set up a Stats object so we can get the details for the
			// pull request.
			//
			// TODO: This is a bit clunky. Can we improve it without
			// forcing the low level report code to know all about
			// everything?
			all := stats.Bucket{
				Rule: func(prd *stats.PullRequestDetails) bool {
					return true
				},
			}
			reportBuckets := []*stats.Bucket{
				&all,
			}
			theStats := &stats.Stats{
				Query:        query,
				EarliestDate: earliestDate,
				Buckets:      reportBuckets,
			}
			if err := theStats.ProcessOne(pr); err != nil {
				return fmt.Errorf("failed to fetch details for PR %d: %w", prID, err)
			}

			summarizer, err := enhancements.NewSummarizer()
			if err != nil {
				return fmt.Errorf("unable to show PR summaries: %w", err)
			}

			report.ShowPRs(
				summarizer,
				fmt.Sprintf("Pull Request %d", prID),
				all.Requests,
				true,
				true,
			)

			prd := all.Requests[0]

			var sinceUpdated float64
			var sinceClosed float64

			if prd.Pull.UpdatedAt != nil && !prd.Pull.UpdatedAt.IsZero() {
				sinceUpdated = time.Since(*prd.Pull.UpdatedAt).Hours() / 24
			}
			if prd.Pull.ClosedAt != nil && !prd.Pull.ClosedAt.IsZero() {
				sinceClosed = time.Since(*prd.Pull.ClosedAt).Hours() / 24
			}

			fmt.Printf("Last updated:   %s (%.02f days)\n", prd.Pull.UpdatedAt, sinceUpdated)
			fmt.Printf("Closed:         %s (%.02f days)\n", prd.Pull.ClosedAt, sinceClosed)
			fmt.Printf("Group:          %s\n", prd.Group)
			fmt.Printf("Enhancement:    %v\n", prd.IsEnhancement)
			fmt.Printf("State:          %q\n", prd.State)
			fmt.Printf("LGTM:           %v\n", prd.LGTM)
			fmt.Printf("Prioritized:    %v\n", prd.Prioritized)
			fmt.Printf("Stale:          %v\n", prd.Stale)
			fmt.Printf("Reviews:        %3d / %3d\n", prd.RecentReviewCount, len(prd.Reviews))
			fmt.Printf("PR Comments:    %3d / %3d\n", prd.RecentPRCommentCount, len(prd.PullRequestComments))
			fmt.Printf("Issue comments: %3d / %3d\n", prd.RecentIssueCommentCount, len(prd.IssueComments))
			fmt.Printf("Is New:         %v\n", prd.IsNew)
			fmt.Printf("Modified files:\n")
			for _, file := range prd.ModifiedFiles {
				fmt.Printf("\t(%s) %s\n", file.Mode, file.Name)
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&daysBack, "days-back", 7, "how many days back to query")

	return cmd
}

func init() {
	rootCmd.AddCommand(newShowPRCommand())
}
