# GitHub To GitLab CI Mirror

This project uses a free GitHub Actions workflow to mirror pushes from GitHub to GitLab.

Flow:

```text
git push GitHub main
-> GitHub Actions mirrors main to GitLab
-> GitLab CI/CD pipeline starts
-> GitLab Runner deploys to VPS
```

Workflow file:

```text
.github/workflows/mirror-to-gitlab.yml
```

## GitLab Token

Create a GitLab token with repository write access:

`GitLab -> Project -> Settings -> Access Tokens`

Recommended token:

```text
Name: github-mirror
Role: Maintainer
Scopes: write_repository
```

Copy the token once. Do not commit it.

## GitHub Secrets

Add these in:

`GitHub -> Repository -> Settings -> Secrets and variables -> Actions -> New repository secret`

| Secret | Example |
| --- | --- |
| `GITLAB_PROJECT_PATH` | `namespace/project` |
| `GITLAB_TOKEN` | GitLab project access token |

For `GITLAB_PROJECT_PATH`, use the path from the GitLab project URL:

```text
https://gitlab.com/namespace/project
```

Only this part goes into the secret:

```text
namespace/project
```

## Important

GitLab becomes a mirror target. Do not make direct commits in GitLab unless you also push them back to GitHub, because the next GitHub push expects GitLab `main` to be a fast-forward update from GitHub.

If the workflow fails with a non-fast-forward error, GitLab `main` has commits that are not in GitHub. In that case, either merge those commits back into GitHub or reset GitLab `main` once.
