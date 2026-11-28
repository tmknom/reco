package show

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks FR-004 QR-002
func TestWriteObservationResult(t *testing.T) {
	target := mustReference(t, "owner/repo")
	allPolicy := &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(setting.AllowedActionsAll),
		SHAPinningRequired: github.Observed(true),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
	}
	readOnly := &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionRead), CanApprovePullRequestReviews: false}
	notShared := &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessNone)}
	privateApproval := &github.ForkPullRequestApproval{ApprovalPolicy: github.Unavailable[setting.ForkPullRequestApprovalPolicy](github.UnavailablePrivateRepository)}
	noForkWorkflows := &github.PrivateForkPullRequestWorkflows{Conditions: github.Observed(setting.PrivateForkPullRequestWorkflowConditions{})}
	// 保持期間と上限を違う値にし、取り違えと、上限を値の位置に書く誤りを検出する
	retention := &github.Retention{Days: 30, MaximumAllowedDays: 400}
	defaultOIDCSubject := &github.OIDCSubject{Customization: setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(false)}}
	noVariables := &github.Variables{}
	noEnvironments := &github.Environments{}
	enabledDependabot := &github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled}
	enabledSecretScanning := &github.SecretScanning{Enabled: github.Observed(true), PushProtection: github.Observed(true)}
	disabledPrivateVulnerabilityReporting := &github.PrivateVulnerabilityReporting{Enabled: github.Observed(false)}
	emptyRulesets := &github.Rulesets{Existing: github.Observed([]github.Ruleset{})}
	// General の値ごとの表示は writeGeneral のテストが確かめる。ここでは、どのケースも GitHub が返し得る同じ値にする
	general := github.General{
		DeleteBranchOnMerge: true, AllowSquashMerge: true,
		SquashMergeCommitDefault: github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages}),
		MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
		WebCommitSignoffRequired: github.Observed(false),
	}
	const (
		noRulesets   = "    # rulesets: none\n"
		header       = "repositories:\n  - name: owner/repo\n"
		generalLines = "    general:\n      delete_branch_on_merge: true\n      allow_auto_merge: false\n" +
			"      allow_squash_merge: true\n      allow_merge_commit: false\n      allow_rebase_merge: false\n      allow_update_branch: false\n" +
			"      squash_merge_commit:\n        title: COMMIT_OR_PR_TITLE\n        message: COMMIT_MESSAGES\n" +
			"      merge_commit:\n        title: MERGE_MESSAGE\n        message: PR_TITLE\n" +
			"      has_issues: false\n      has_projects: false\n      has_wiki: false\n      web_commit_signoff_required: false\n"
		allPermissions                      = "    actions:\n      permissions:\n        enabled: true\n        allowed_actions: all\n        sha_pinning_required: true\n"
		readOnlyWorkflow                    = "      workflow:\n        default_workflow_permissions: read\n        can_approve_pull_request_reviews: false\n"
		notSharedAccess                     = "      access:\n        access_level: none\n"
		privateForkPullRequestsAndRetention = "      fork_pull_requests:\n        # approval_policy: unavailable (the repository is private)\n        private_repositories:\n" +
			"          run_workflows_from_fork_pull_requests: false\n          send_write_tokens_to_workflows: false\n" +
			"          send_secrets_and_variables: false\n          require_approval_for_fork_pr_workflows: false\n" +
			"      retention:\n        days: 30 # maximum allowed: 400\n"
		defaultOIDCSubjectLines = "      oidc_subject:\n        use_default: true\n        use_immutable_subject: false\n"
		noVariable              = "      # variables: none\n"
		noEnvironment           = "    # environments: none\n"
		enabledSecurity         = "    security:\n      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
			"      secret_scanning: true\n      secret_scanning_push_protection: true\n      private_vulnerability_reporting: false\n"
	)
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     string
	}{
		{
			name: "all areas are shown in the configuration structure and the all policy omits the criteria",
			observed: github.ObservationResult{
				RepositoryID:                    1,
				General:                         general,
				ActionsPermissions:              allPolicy,
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header +
				generalLines +
				allPermissions + readOnlyWorkflow + notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "opposite workflow and access values are shown as observed",
			observed: github.ObservationResult{
				RepositoryID:                    1,
				General:                         general,
				ActionsPermissions:              allPolicy,
				WorkflowPermissions:             &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionWrite), CanApprovePullRequestReviews: true},
				ActionsAccess:                   &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessUser)},
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header +
				generalLines +
				allPermissions +
				"      workflow:\n        default_workflow_permissions: write\n        can_approve_pull_request_reviews: true\n" +
				"      access:\n        access_level: user\n" +
				privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "selected policy shows the three criteria with one quoted pattern per line",
			observed: github.ObservationResult{
				RepositoryID: 1,
				General:      general,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Observed(setting.AllowedActionsSelected),
					SHAPinningRequired: github.Observed(false),
					SelectedActions: github.Observed(setting.SelectedActionsCriteria{
						GitHubOwnedAllowed: true,
						VerifiedAllowed:    false,
						PatternsAllowed:    []string{"actions/checkout@v4", "example/*"},
					}),
				},
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines +
				"    actions:\n      permissions:\n        enabled: true\n        allowed_actions: selected\n        selected_actions:\n" +
				"          github_owned_allowed: true\n          verified_allowed: false\n          patterns_allowed:\n" +
				"            - \"actions/checkout@v4\"\n            - \"example/*\"\n        sha_pinning_required: false\n" +
				readOnlyWorkflow + notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "selected policy without patterns shows the empty sequence",
			observed: github.ObservationResult{
				RepositoryID: 1,
				General:      general,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Observed(setting.AllowedActionsSelected),
					SHAPinningRequired: github.Observed(false),
					SelectedActions:    github.Observed(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true}),
				},
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines +
				"    actions:\n      permissions:\n        enabled: true\n        allowed_actions: selected\n        selected_actions:\n" +
				"          github_owned_allowed: true\n          verified_allowed: true\n          patterns_allowed: []\n        sha_pinning_required: false\n" +
				readOnlyWorkflow + notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "disabled actions shows only enabled without the policy, the criteria, and sha_pinning_required",
			observed: github.ObservationResult{
				RepositoryID: 1,
				General:      general,
				ActionsPermissions: &github.ActionsPermissions{
					AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
					SHAPinningRequired: github.Unavailable[bool](github.UnavailableActionsDisabled),
					SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
				},
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines +
				"    actions:\n      permissions:\n        enabled: false\n" +
				readOnlyWorkflow + notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "public repository shows access and the unavailable access level as comments",
			observed: github.ObservationResult{
				RepositoryID:            1,
				General:                 general,
				ActionsPermissions:      allPolicy,
				WorkflowPermissions:     readOnly,
				ActionsAccess:           &github.ActionsAccess{AccessLevel: github.Unavailable[setting.AccessLevel](github.UnavailablePublicRepository)},
				ForkPullRequestApproval: &github.ForkPullRequestApproval{ApprovalPolicy: github.Observed(setting.ForkPullRequestApprovalFirstTimeContributors)},
				PrivateForkPullRequestWorkflows: &github.PrivateForkPullRequestWorkflows{
					Conditions: github.Unavailable[setting.PrivateForkPullRequestWorkflowConditions](github.UnavailablePublicRepository),
				},
				Retention:                     retention,
				OIDCSubject:                   defaultOIDCSubject,
				RepositoryVariables:           noVariables,
				Environments:                  noEnvironments,
				Dependabot:                    enabledDependabot,
				SecretScanning:                enabledSecretScanning,
				PrivateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
				Rulesets:                      emptyRulesets,
			},
			want: header + generalLines + allPermissions + readOnlyWorkflow +
				"      # access:\n      #   access_level: unavailable (the repository is public)\n" +
				"      fork_pull_requests:\n        approval_policy: first_time_contributors\n" +
				"        # private_repositories: unavailable (the repository is public)\n" +
				"      retention:\n        days: 30 # maximum allowed: 400\n" +
				defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "organization access level is shown as observed",
			observed: github.ObservationResult{
				RepositoryID:                    1,
				General:                         general,
				ActionsPermissions:              allPolicy,
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessOrganization)},
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines + allPermissions + readOnlyWorkflow +
				"      access:\n        access_level: organization\n" +
				privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "an unknown allowed_actions value is a comment, selected_actions is omitted, and sha_pinning_required is shown",
			observed: github.ObservationResult{
				RepositoryID: 1,
				General:      general,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableUnknownValue),
					SHAPinningRequired: github.Observed(true),
					SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableUnknownValue),
				},
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines +
				"    actions:\n      permissions:\n        enabled: true\n" +
				"        # allowed_actions: unavailable (GitHub returned a value that reco does not know)\n" +
				"        sha_pinning_required: true\n" +
				readOnlyWorkflow + notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "an unknown default_workflow_permissions value is a comment and can_approve_pull_request_reviews is shown",
			observed: github.ObservationResult{
				RepositoryID:       1,
				General:            general,
				ActionsPermissions: allPolicy,
				WorkflowPermissions: &github.WorkflowPermissions{
					DefaultWorkflowPermissions:   github.Unavailable[setting.PermissionLevel](github.UnavailableUnknownValue),
					CanApprovePullRequestReviews: true,
				},
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines + allPermissions +
				"      workflow:\n        # default_workflow_permissions: unavailable (GitHub returned a value that reco does not know)\n        can_approve_pull_request_reviews: true\n" +
				notSharedAccess + privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "an unknown access_level value comments out the whole access key",
			observed: github.ObservationResult{
				RepositoryID:                    1,
				General:                         general,
				ActionsPermissions:              allPolicy,
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   &github.ActionsAccess{AccessLevel: github.Unavailable[setting.AccessLevel](github.UnavailableUnknownValue)},
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        emptyRulesets,
			},
			want: header + generalLines + allPermissions + readOnlyWorkflow +
				"      # access:\n      #   access_level: unavailable (GitHub returned a value that reco does not know)\n" +
				privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + noRulesets,
		},
		{
			name: "rulesets are shown after security",
			observed: github.ObservationResult{
				RepositoryID:                    1,
				General:                         general,
				ActionsPermissions:              allPolicy,
				WorkflowPermissions:             readOnly,
				ActionsAccess:                   notShared,
				ForkPullRequestApproval:         privateApproval,
				PrivateForkPullRequestWorkflows: noForkWorkflows,
				Retention:                       retention,
				OIDCSubject:                     defaultOIDCSubject,
				RepositoryVariables:             noVariables,
				Environments:                    noEnvironments,
				Dependabot:                      enabledDependabot,
				SecretScanning:                  enabledSecretScanning,
				PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
				Rulesets:                        &github.Rulesets{Existing: github.Unavailable[[]github.Ruleset](github.UnavailablePrivateRepository)},
			},
			want: header + generalLines + allPermissions + readOnlyWorkflow + notSharedAccess +
				privateForkPullRequestsAndRetention + defaultOIDCSubjectLines + noVariable + noEnvironment + enabledSecurity + "    # rulesets: unavailable (the repository is private)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeObservationResult(&out, target, tc.observed); err != nil {
				t.Fatalf("writeObservationResult() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeObservationResult() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		observed := github.ObservationResult{
			General:                         general,
			ActionsPermissions:              allPolicy,
			WorkflowPermissions:             readOnly,
			ActionsAccess:                   notShared,
			ForkPullRequestApproval:         privateApproval,
			PrivateForkPullRequestWorkflows: noForkWorkflows,
			Retention:                       retention,
			OIDCSubject:                     defaultOIDCSubject,
			RepositoryVariables:             noVariables,
			Environments:                    noEnvironments,
			Dependabot:                      enabledDependabot,
			SecretScanning:                  enabledSecretScanning,
			PrivateVulnerabilityReporting:   disabledPrivateVulnerabilityReporting,
			Rulesets:                        emptyRulesets,
		}
		err := writeObservationResult(failingWriter{}, target, observed)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeObservationResult() error = %v, want %v", err, errWrite)
		}
	})
}

// TestWriteActionsSettings_UndefinedReasonPanics は、観測が作らない理由の観測不能を、ほかの理由の文言で示さず、行を黙って省きもしないことを確かめる。
// 対象の関数を直接呼び、ほかの行の書き込みが起こす panic と区別する。
func TestWriteActionsSettings_UndefinedReasonPanics(t *testing.T) {
	cases := []struct {
		name  string
		write func(b *strings.Builder)
	}{
		{name: "allowed_actions unavailable for a reason other than disabled actions or an unknown value panics", write: func(b *strings.Builder) {
			writeActionsPermissions(b, github.ActionsPermissions{
				Enabled:            true,
				AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailablePublicRepository),
				SHAPinningRequired: github.Observed(false),
				SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailablePublicRepository),
			})
		}},
		{name: "default_workflow_permissions unavailable for a reason other than an unknown value panics", write: func(b *strings.Builder) {
			writeWorkflowPermissions(b, github.WorkflowPermissions{DefaultWorkflowPermissions: github.Unavailable[setting.PermissionLevel](github.UnavailableActionsDisabled)})
		}},
		{name: "access level unavailable for a reason other than public or an unknown value panics", write: func(b *strings.Builder) {
			writeActionsAccess(b, github.ActionsAccess{AccessLevel: github.Unavailable[setting.AccessLevel](github.UnavailableActionsDisabled)})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("did not panic")
				}
			}()
			tc.write(&strings.Builder{})
		})
	}
}

const (
	repositoryPath                      = "/repos/owner/repo"
	actionsPermissionsPath              = "/repos/owner/repo/actions/permissions"
	workflowPermissionsPath             = "/repos/owner/repo/actions/permissions/workflow"
	actionsAccessPath                   = "/repos/owner/repo/actions/permissions/access"
	privateForkPullRequestWorkflowsPath = "/repos/owner/repo/actions/permissions/fork-pr-workflows-private-repos"
	retentionPath                       = "/repos/owner/repo/actions/permissions/artifact-and-log-retention"
	oidcSubjectPath                     = "/repos/owner/repo/actions/oidc/customization/sub"
	variablesPath                       = "/repos/owner/repo/actions/variables"
	environmentsPath                    = "/repos/owner/repo/environments"
	vulnerabilityAlertsPath             = "/repos/owner/repo/vulnerability-alerts"
	securityFixesPath                   = "/repos/owner/repo/automated-security-fixes"
)

//trace:checks FR-004
func TestRun(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("all areas are observed with GET only and written to Out", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:                      githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, Private: true}.JSON()),
			"GET " + actionsPermissionsPath:              githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "local_only"}.JSON()),
			"GET " + workflowPermissionsPath:             githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "write", CanApprovePullRequestReviews: true}.JSON()),
			"GET " + actionsAccessPath:                   githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "user"}.JSON()),
			"GET " + privateForkPullRequestWorkflowsPath: githubtest.OK(githubtest.PrivateForkPullRequestWorkflowsBody{}.JSON()),
			"GET " + retentionPath:                       githubtest.OK(githubtest.RetentionBody{Days: 90, MaximumAllowedDays: 400}.JSON()),
			"GET " + oidcSubjectPath:                     githubtest.OK(githubtest.OIDCSubjectBody{IncludeClaimKeys: []string{"repo"}}.JSON()),
			"GET " + variablesPath: githubtest.OK(githubtest.VariablesBody{TotalCount: 1, Variables: []githubtest.VariableBody{
				{Name: "GO_VERSION", Value: "1.27"},
			}}.JSON()),
			"GET " + environmentsPath: githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{
				{ID: 2, Name: "staging"},
			}}.JSON()),
			"GET " + environmentsPath + "/staging/variables": githubtest.OK(githubtest.VariablesBody{TotalCount: 1, Variables: []githubtest.VariableBody{
				{Name: "REGION", Value: "eu"},
			}}.JSON()),
			"GET " + vulnerabilityAlertsPath: {Status: http.StatusNoContent},
			"GET " + securityFixesPath:       githubtest.OK(githubtest.AutomatedSecurityFixesBody{Enabled: true, Paused: true}.JSON()),
		}))
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(server.URL())}}

		if err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		// 非公開リポジトリのため、承認ポリシーの取得は送らない
		want := []string{"GET " + repositoryPath, "GET " + actionsPermissionsPath, "GET " + workflowPermissionsPath, "GET " + actionsAccessPath,
			"GET " + privateForkPullRequestWorkflowsPath, "GET " + retentionPath, "GET " + oidcSubjectPath, "GET " + variablesPath, "GET " + environmentsPath,
			"GET " + environmentsPath + "/staging/variables", "GET " + vulnerabilityAlertsPath, "GET " + securityFixesPath}
		if got := server.RequestLines(); !slices.Equal(got, want) {
			t.Errorf("got requests %v, want %v", got, want)
		}
		for _, r := range server.Requests() {
			if !strings.Contains(r.Authorization, "test-token") {
				t.Errorf("got Authorization %q, want it to contain the token", r.Authorization)
			}
		}
		for _, observed := range []string{
			"default_workflow_permissions: write", `        - name: "GO_VERSION"`, `- name: "staging"`, `          - name: "REGION"`, "dependabot_security_updates: true # paused",
			"        # approval_policy: unavailable (the repository is private)",
			"        days: 90 # maximum allowed: 400",
			"        include_claim_keys:\n          - \"repo\"",
			"# secret_scanning: unavailable (the repository is private)",
			"# private_vulnerability_reporting: unavailable (the repository is private)",
			"    # rulesets: unavailable (the repository is private)",
		} {
			if !strings.Contains(out.String(), observed) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), observed)
			}
		}
	})

	failures := []struct {
		name       string
		failedPath string
	}{
		{name: "actions permissions failure is returned as an error instead of omitting the settings", failedPath: actionsPermissionsPath},
		{name: "workflow permissions failure is returned without writing to Out", failedPath: workflowPermissionsPath},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			// 失敗した取得より後の領域は登録しない。失敗のあとに観測を続ければ、stub が登録のない要求として報告する
			routes := map[string]githubtest.Response{
				"GET " + repositoryPath:         githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, Admin: true}.JSON()),
				"GET " + actionsPermissionsPath: githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "all"}.JSON()),
			}
			routes["GET "+tc.failedPath] = githubtest.Response{Status: http.StatusForbidden}
			server := githubtest.NewServer(t, githubtest.Routes(t, routes))
			var out bytes.Buffer
			cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(server.URL())}}

			err := Run(context.Background(), cfg)
			if !errors.Is(err, github.ErrForbidden) {
				t.Errorf("Run() error = %v, want %v", err, github.ErrForbidden)
			}
			if out.Len() != 0 {
				t.Errorf("got Out %q, want empty", out.String())
			}
		})
	}
}

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}
