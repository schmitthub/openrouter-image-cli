#!/usr/bin/env bash
#
# setup-repo-rulesets.sh — Create/update the repository rulesets that protect
# main and release tags. Idempotent: re-running updates the existing rulesets
# in place (matched by name).
#
# Rulesets created:
#   protect-main      — main branch: PRs required (0 approvals — solo repo),
#                       required status checks (PR workflow jobs), no force
#                       pushes, no deletion. Repo admins can bypass.
#   protect-release-tags — v* tags: only repo admins can create; nobody can
#                       update, delete, or force-push them. Pairs with the
#                       release workflow's semver/on-main/CI-green validation.
#
# Usage: bash scripts/setup-repo-rulesets.sh
# Requires: gh (authenticated with admin rights on the repo)
#
set -euo pipefail

REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
echo "Configuring rulesets for ${REPO}"

# upsert_ruleset NAME JSON — POST a new ruleset, or PUT over the existing one
# with the same name.
upsert_ruleset() {
    local name="$1" json="$2" existing_id
    existing_id=$(gh api "repos/${REPO}/rulesets" --jq \
        ".[] | select(.name == \"${name}\") | .id" 2>/dev/null || true)
    if [[ -n "${existing_id}" ]]; then
        echo "Updating ruleset '${name}' (id ${existing_id})"
        gh api -X PUT "repos/${REPO}/rulesets/${existing_id}" --input - <<<"${json}" >/dev/null
    else
        echo "Creating ruleset '${name}'"
        gh api -X POST "repos/${REPO}/rulesets" --input - <<<"${json}" >/dev/null
    fi
}

# Required status checks = the PR workflow's job names (reusable workflow jobs
# surface as "<caller job name> / <inner job name>"). Update this list when
# .github/workflows/pr.yml changes. integration_id 15368 = GitHub Actions.
upsert_ruleset "protect-main" '{
  "name": "protect-main",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": { "include": ["~DEFAULT_BRANCH"], "exclude": [] }
  },
  "bypass_actors": [
    { "actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always" }
  ],
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 0,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false,
        "allowed_merge_methods": ["squash", "merge"]
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [
          { "context": "Lint / golangci-lint", "integration_id": 15368 },
          { "context": "Test / Unit Tests", "integration_id": 15368 },
          { "context": "Security / Semgrep SAST", "integration_id": 15368 },
          { "context": "Security / Gitleaks Secrets", "integration_id": 15368 },
          { "context": "Security / govulncheck SCA", "integration_id": 15368 }
        ]
      }
    }
  ]
}'

# Tag ruleset: "creation" with only admins in bypass_actors means only admins
# can push v* tags; deletion/non_fast_forward/update lock published tags so a
# release tag can never be silently re-pointed at different code.
upsert_ruleset "protect-release-tags" '{
  "name": "protect-release-tags",
  "target": "tag",
  "enforcement": "active",
  "conditions": {
    "ref_name": { "include": ["refs/tags/v*"], "exclude": [] }
  },
  "bypass_actors": [
    { "actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always" }
  ],
  "rules": [
    { "type": "creation" },
    { "type": "update" },
    { "type": "deletion" },
    { "type": "non_fast_forward" }
  ]
}'

echo ""
echo "Done. Verify: gh api repos/${REPO}/rulesets --jq '.[].name'"
