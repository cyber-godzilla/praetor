# Release-to-wiki automation

The tag-triggered release workflow updates the release-history table in
`Eternal-City-Community/tec-wiki` after it publishes a Praetor release. It
creates or reuses an `automation/praetor-<tag>-release-history` branch and opens
a pull request; it never pushes directly to the wiki's `main` branch and never
force-pushes.

## Required secret

Add `TEC_WIKI_PR_TOKEN` as a Praetor Actions secret. Use a fine-grained token
owned by `cyber-godzilla`, limited to the `Eternal-City-Community/tec-wiki`
repository, with these repository permissions:

- **Contents:** Read and write
- **Pull requests:** Read and write

Praetor's normal `GITHUB_TOKEN` cannot write to another repository, so the
cross-repository token is required.

## Release summary selection

Before creating a tag, add `.github/release-summaries/<tag>.md` with one short
paragraph describing the release's major user-facing changes. The release job
requires that file and prepends it to the generated GitHub notes as an explicit
wiki summary. For example, v0.5.0 uses
`.github/release-summaries/v0.5.0.md`.

The updater then chooses up to three concise items in this order:

1. An explicit release-note comment:
   `<!-- wiki-release-summary: Major user-facing changes. -->`
2. A `## Wiki release summary` section.
3. Bullets under the generated `## What's Changed` section.
4. Meaningful feature, fix, performance, or documentation commit subjects
   between the previous tag and the new tag.

If none of those provides a usable summary, the wiki job fails rather than
publishing an invented or empty history entry. Edit the GitHub release notes to
add the explicit comment, then rerun the failed job.
