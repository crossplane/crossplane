# Crossplane PR Approval Guidelines & Issue Commitment Plan

## Goal
Establish a comprehensive, implementation-ready plan detailing how Pull Requests (PRs) are approved properly according to Crossplane's contributing guides, and categorize all the issue types and areas you can commit to.

---

## 1. PR Approval Requirements & Code Review Process

To ensure a PR is approved and merged properly in Crossplane, all contributions must satisfy the guidelines defined in `contributing/README.md`:

### A. Pre-Contribution & Intent
- **GitHub Issue Discussion:** Communicate your intent by raising or commenting on a GitHub issue before starting implementation to avoid duplicated effort or rejected approaches.
- **Commit Hygiene & DCO:** 
  - Follow good git commit history (tell a story, break up changes into logical commits).
  - Sign off on all commits using `git commit -s` (Developer Certificate of Origin / DCO enforcement by bot).
  - Do not force push to address review feedback; clean up commit history after approval if needed.

### B. Testing Standards
- **Unit Tests:** Most PRs touching code must include unit tests aiming for ~80% coverage.
  - Must use standard Go `testing` package and table-driven tests.
  - **Forbidden:** Third-party test frameworks like Ginkgo, Gomega, or Testify.
  - Use `google/go-cmp` and `crossplane-runtime/pkg/test` (e.g., `cmpopts.EquateErrors`, `cmpopts.AnyError`).
- **End-to-End (E2E) Tests:** Significant features must be covered by E2E tests (`test/e2e`).

### C. Coding Style & Linting
- Follow `Effective Go`, Go Code Review Comments, and Test Comments.
- Return early, avoid `else` where possible, wrap errors with `crossplane-runtime/pkg/errors` using inline strings.
- Explain any `//nolint` directives with specific linter names and rationale.
- Run linters and checks locally via `./nix.sh run .#lint` and `./nix.sh flake check`.

### D. Documentation & UX
- Any change introducing new behavior or modifying existing behavior must include updates to documentation in the docs repository.
- Keep documentation changes in distinct commits.
- Conditions and events must be actionable, stable, deterministic, and non-flapping.

### E. Approvals & CI Check Pass
- **Required Approvals:**
  - At least one approval from [Reviewers] (`OWNERS.md#reviewers`).
  - At least one approval from [Maintainers] (`OWNERS.md#maintainers`).
- **CI Checks:**
  - All CI jobs must pass, including `./nix.sh flake check`, linter, `checklist-completed`, and DCO.

---

## 2. Issues and Contribution Categories You Can Commit To (Primary Focus: Good First Issues)

According to the contributing guide and your preference, you will focus initially on **Good First Issues** while understanding the broader ecosystem options:

### Selected Focus: Good First Issues (Crossplane Core)
- **Definition & Target:** Low-complexity issues in `crossplane/crossplane` labeled `good first issue`.
- **Workflow for Good First Issues:**
  1. Browse open issues with the `good first issue` label on GitHub.
  2. Comment on the issue expressing your intent to work on it and asking to be assigned.
  3. Ensure you follow all testing (`testing` package, table-driven tests), DCO (`git commit -s`), and PR review guidelines when submitting your PR.

---

## 3. Execution & Verification Workflow

1. **Setup Development Environment:**
   ```sh
   ./nix.sh run .#test
   ```
2. **Implement Changes:**
   - Write clean, idiomatic Go code following table-driven test patterns.
   - Run tests and linters:
     ```sh
     ./nix.sh run .#test
     ./nix.sh run .#lint
     ./nix.sh flake check
     ```
3. **Commit & Sign-off:**
   ```sh
   git commit -s -m "Descriptive commit message"
   ```
4. **Open PR & Address Review:**
   - Fill out PR checklist and template.
   - Address reviewer feedback in subsequent commits without force-pushing.
   - Secure approvals from a Reviewer and a Maintainer.
