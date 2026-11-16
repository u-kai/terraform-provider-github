package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccGithubRepositoryCollaborators(t *testing.T) {
	t.Parallel()

	t.Run("with_teams_and_users", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)
		skipUnlessHasOrgUser1(t)

		team0 := mustCreateTestTeam(t)
		mustAssignOrganizationRoleToTeam(t, team0, 138)

		repo := mustCreateTestRepository(t)
		team1 := mustCreateTestTeam(t)
		mustAddRepositoryToTeam(t, team1, repo)
		_ = mustCreateTestTeam(t, withNewTeamParent(team1.GetID()))

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  team {
    team_id    = "%v"
    permission = "push"
  }

  user {
    username = "%v"
    permission = "pull"
  }
}
`, repo.GetName(), team1.GetSlug(), testAccConf.testOrgUser1)

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config: config,
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					ResourceName:      "github_repository_collaborators.test",
					ImportState:       true,
					ImportStateId:     repo.GetName(),
					ImportStateVerify: true,
				},
			},
		})
	})

	t.Run("with_only_teams", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)

		team0 := mustCreateTestTeam(t)
		mustAssignOrganizationRoleToTeam(t, team0, 138)

		repo := mustCreateTestRepository(t)
		team1 := mustCreateTestTeam(t)
		mustAddRepositoryToTeam(t, team1, repo)
		team2 := mustCreateTestTeam(t)
		mustAddRepositoryToTeam(t, team2, repo)
		_ = mustCreateTestTeam(t, withNewTeamParent(team1.GetID()))

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  team {
    team_id    = "%%v"
    permission = "%%v"
  }

  team {
    team_id = "%%v"
  }
}
`, repo.GetName())

		configRemoveTeam := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  team {
    team_id    = "%%v"
    permission = "%%v"
  }
}
`, repo.GetName())

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(config, team1.GetID(), "pull", team2.GetID()),
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					Config: fmt.Sprintf(config, team1.GetSlug(), "pull", team2.GetSlug()),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					Config: fmt.Sprintf(config, team1.GetSlug(), "push", team2.GetSlug()),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					ResourceName:      "github_repository_collaborators.test",
					ImportState:       true,
					ImportStateId:     repo.GetName(),
					ImportStateVerify: true,
				},
				{
					Config: fmt.Sprintf(configRemoveTeam, team1.GetSlug(), "push"),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
			},
		})
	})

	t.Run("with_only_users", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)
		skipUnlessHasOrgUser1(t)
		skipUnlessHasOrgUser2(t)

		team0 := mustCreateTestTeam(t)
		mustAssignOrganizationRoleToTeam(t, team0, 138)

		repo := mustCreateTestRepository(t)

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  user {
    username   = "%%v"
    permission = "%%v"
  }

  user {
    username = "%%v"
  }
}
`, repo.GetName())

		configRemoveUser := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  user {
    username   = "%%v"
    permission = "%%v"
  }
}
`, repo.GetName())

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(config, testAccConf.testOrgUser1, "pull", testAccConf.testOrgUser2),
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					Config: fmt.Sprintf(config, testAccConf.testOrgUser1, "push", testAccConf.testOrgUser2),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					ResourceName:      "github_repository_collaborators.test",
					ImportState:       true,
					ImportStateId:     repo.GetName(),
					ImportStateVerify: true,
				},
				{
					Config: fmt.Sprintf(configRemoveUser, testAccConf.testOrgUser1, "push"),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					SkipFunc: func() (bool, error) {
						if len(testAccConf.testExternalUser1) == 0 {
							return true, nil
						}
						return false, nil
					},
					Config: fmt.Sprintf(config, testAccConf.testOrgUser1, "push", testAccConf.testExternalUser1),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionUpdate),
						},
					},
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(1)),
					},
				},
			},
		})
	})

	t.Run("with_user_repo", func(t *testing.T) {
		t.Parallel()

		skipUnlessMode(t, individual)

		repo := mustCreateTestRepository(t)

		configNoUser := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"
}
`, repo.GetName())

		configWithUser := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  user {
    username   = "%v"
    permission = "admin"
  }
}
`, repo.GetName(), testAccConf.owner)

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config: configNoUser,
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(false)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
				{
					ResourceName:      "github_repository_collaborators.test",
					ImportState:       true,
					ImportStateId:     repo.GetName(),
					ImportStateVerify: true,
				},
				{
					Config: configWithUser,
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("repository_id"), knownvalue.Int32Exact(int32(repo.GetID()))),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("owner_configured"), knownvalue.Bool(true)),
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("invitation_ids"), knownvalue.MapSizeExact(0)),
					},
				},
			},
		})
	})

	t.Run("errors_with_duplicate_teams", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)

		repo := mustCreateTestRepository(t)
		team1 := mustCreateTestTeam(t)

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  team {
    team_id    = "%v"
    permission = "pull"
  }

  team {
    team_id    = "%v"
    permission = "push"
  }
}
`, repo.GetName(), team1.GetSlug(), team1.GetSlug())

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config:      config,
					ExpectError: regexp.MustCompile(`duplicate team .+ found`),
				},
			},
		})
	})

	t.Run("errors_with_duplicate_users", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)
		skipUnlessHasOrgUser1(t)

		repo := mustCreateTestRepository(t)

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  user {
    username   = "%v"
    permission = "pull"
  }

  user {
    username   = "%v"
    permission = "push"
  }
}
`, repo.GetName(), testAccConf.testOrgUser1, testAccConf.testOrgUser1)

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config:      config,
					ExpectError: regexp.MustCompile(`duplicate user .+ found`),
				},
			},
		})
	})

	t.Run("is_case_insensitive", func(t *testing.T) {
		t.Parallel()

		skipUnlessHasOrgs(t)
		skipUnlessHasOrgUser1(t)

		repo := mustCreateTestRepository(t)

		config := fmt.Sprintf(`
resource "github_repository_collaborators" "test" {
  repository = "%v"

  user {
    username   = "%%v"
    permission = "pull"
  }
}
`, repo.GetName())

		resource.Test(t, resource.TestCase{
			ProviderFactories: providerFactories,
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(config, flipUsernameCase(testAccConf.testOrgUser1)),
					ConfigStateChecks: []statecheck.StateCheck{
						statecheck.ExpectKnownValue("github_repository_collaborators.test", tfjsonpath.New("user"), knownvalue.SetSizeExact(1)),
					},
				},
				{
					Config: fmt.Sprintf(config, testAccConf.testOrgUser1),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("github_repository_collaborators.test", plancheck.ResourceActionNoop),
						},
					},
				},
			},
		})
	})
}

func TestGithubRepositoryCollaboratorsUserDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		configUsername string
		configPerm     string
		wantUserDiff   bool
	}{
		{name: "same case", configUsername: "FooBar", configPerm: "push", wantUserDiff: false},
		{name: "username differs only by case", configUsername: "foobar", configPerm: "push", wantUserDiff: false},
		{name: "permission changed", configUsername: "foobar", configPerm: "pull", wantUserDiff: true},
		{name: "different user", configUsername: "foobaz", configPerm: "push", wantUserDiff: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := resourceGithubRepositoryCollaborators()
			// The CustomizeDiff needs a configured provider; only the set identity is under test.
			r.CustomizeDiff = nil

			user := func(username, permission string) map[string]any {
				return map[string]any{
					"repository": "repo",
					"user":       []any{map[string]any{"username": username, "permission": permission}},
				}
			}

			state := schema.TestResourceDataRaw(t, r.Schema, user("FooBar", "push"))
			state.SetId("1")

			diff, err := r.SimpleDiff(t.Context(), state.State(), terraform.NewResourceConfigRaw(user(tt.configUsername, tt.configPerm)), nil)
			if err != nil {
				t.Fatal(err)
			}

			gotUserDiff := false
			if diff != nil {
				for k := range diff.Attributes {
					gotUserDiff = gotUserDiff || strings.HasPrefix(k, "user.")
				}
			}
			if gotUserDiff != tt.wantUserDiff {
				t.Errorf("user diff = %v, want %v; diff: %v", gotUserDiff, tt.wantUserDiff, diff)
			}
		})
	}
}

func TestUpdateUserCollaboratorsAndInvites(t *testing.T) {
	t.Parallel()

	const (
		collaboratorsPath = "/repos/owner/repo/collaborators"
		invitationsPath   = "/repos/owner/repo/invitations"
	)

	tests := []struct {
		name          string
		collaborators string
		invitations   string
		inUsers       userCollaborators
		wantMutations []string
	}{
		{
			name:          "collaborator differing only by case is kept",
			collaborators: `[{"login":"FooBar","role_name":"write"}]`,
			invitations:   `[]`,
			inUsers:       userCollaborators{{userIdentity: userIdentity{login: "foobar"}, permission: "push"}},
		},
		{
			name:          "invitation differing only by case is kept",
			collaborators: `[]`,
			invitations:   `[{"id":1,"invitee":{"login":"FooBar"},"permissions":"write"}]`,
			inUsers:       userCollaborators{{userIdentity: userIdentity{login: "foobar"}, permission: "push"}},
		},
		{
			name:          "collaborator differing by case gets permission updated",
			collaborators: `[{"login":"FooBar","role_name":"write"}]`,
			invitations:   `[]`,
			inUsers:       userCollaborators{{userIdentity: userIdentity{login: "foobar"}, permission: "pull"}},
			wantMutations: []string{"PUT " + collaboratorsPath + "/foobar"},
		},
		{
			name:          "invitation differing by case gets permission updated",
			collaborators: `[]`,
			invitations:   `[{"id":1,"invitee":{"login":"FooBar"},"permissions":"write"}]`,
			inUsers:       userCollaborators{{userIdentity: userIdentity{login: "foobar"}, permission: "pull"}},
			wantMutations: []string{"PATCH " + invitationsPath + "/1"},
		},
		{
			name:          "unconfigured collaborator is removed",
			collaborators: `[{"login":"FooBar","role_name":"write"}]`,
			invitations:   `[]`,
			wantMutations: []string{"DELETE " + collaboratorsPath + "/FooBar"},
		},
		{
			name:          "new user is invited",
			collaborators: `[]`,
			invitations:   `[]`,
			inUsers:       userCollaborators{{userIdentity: userIdentity{login: "FooBar"}, permission: "push"}},
			wantMutations: []string{"PUT " + collaboratorsPath + "/FooBar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu        sync.Mutex
				mutations []string
			)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method != http.MethodGet:
					mu.Lock()
					defer mu.Unlock()
					mutations = append(mutations, r.Method+" "+r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				case r.URL.Path == collaboratorsPath && r.URL.Query().Get("affiliation") == "direct":
					_, _ = w.Write([]byte(tt.collaborators))
				case r.URL.Path == invitationsPath:
					_, _ = w.Write([]byte(tt.invitations))
				default:
					_, _ = w.Write([]byte(`[]`))
				}
			}))
			t.Cleanup(ts.Close)

			client, err := github.NewClient(github.WithURLs(new(ts.URL+"/"), nil))
			if err != nil {
				t.Fatal(err)
			}

			meta := &Owner{v3client: client, name: "owner", maxPerPage: 100}
			if _, err := updateUserCollaboratorsAndInvites(context.Background(), meta, "owner", "repo", tt.inUsers, nil); err != nil {
				t.Fatal(err)
			}

			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(mutations, tt.wantMutations) {
				t.Errorf("mutating requests = %v, want %v", mutations, tt.wantMutations)
			}
		})
	}
}
