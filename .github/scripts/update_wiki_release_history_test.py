import sys
import unittest
from pathlib import Path


sys.path.insert(0, str(Path(__file__).parent))
import update_wiki_release_history as updater  # noqa: E402


PAGE = """# Praetor

<!-- praetor-release-history:start -->
| Release | Date | Major features and changes |
| --- | --- | --- |
| [v0.4.15](https://github.com/cyber-godzilla/praetor/releases/tag/v0.4.15) | 2026-09-24 | Existing release. |
<!-- praetor-release-history:end -->
"""


def release(body: str = "", tag: str = "v0.5.0") -> dict[str, object]:
    return {
        "tag_name": tag,
        "html_url": f"https://github.com/cyber-godzilla/praetor/releases/tag/{tag}",
        "published_at": "2026-10-06T14:30:00Z",
        "body": body,
    }


class ReleaseSummaryTests(unittest.TestCase):
    def test_explicit_summary_wins_and_escapes_table_delimiter(self) -> None:
        body = """## What's Changed
* feat: ignored generated note

<!-- wiki-release-summary: Add repeats with count | bounded waits -->
"""
        self.assertEqual(
            updater.release_summary(body, ["feat: ignored commit"]),
            r"Add repeats with count \| bounded waits.",
        )

    def test_generated_notes_strip_attribution(self) -> None:
        body = """## What's Changed
* feat(gui): add the Automation Bar by @cyber-godzilla in https://github.com/cyber-godzilla/praetor/pull/20
* fix: keep scrollback sticky

**Full Changelog**: https://example.invalid
"""
        self.assertEqual(
            updater.release_summary(body, []),
            "Add the Automation Bar. Keep scrollback sticky.",
        )

    def test_commit_fallback_prioritizes_features_and_fixes(self) -> None:
        subjects = [
            "chore(release): bump version to v0.5.0",
            "feat: prepare Praetor 0.5.0",
            "docs: update guide",
            "fix(gui): keep scrollback sticky",
            "feat: add counted repeats",
            "test: cover repeats",
        ]
        self.assertEqual(
            updater.release_summary("**Full Changelog**: https://example.invalid", subjects),
            "Add counted repeats. Keep scrollback sticky. Update guide.",
        )

    def test_generic_release_commit_is_not_accepted_as_a_summary(self) -> None:
        with self.assertRaises(updater.UpdateError):
            updater.release_summary("", ["feat: prepare Praetor 0.5.0"])


class PageUpdateTests(unittest.TestCase):
    def test_inserts_newest_release_after_header(self) -> None:
        row = updater.release_row(
            release("<!-- wiki-release-summary: Add PraetorScript. -->"), []
        )
        updated, changed = updater.update_page(PAGE, row, "v0.5.0")
        self.assertTrue(changed)
        self.assertLess(updated.index("[v0.5.0]"), updated.index("[v0.4.15]"))
        self.assertIn(
            "[v0.5.0](https://github.com/cyber-godzilla/praetor/releases/tag/v0.5.0)",
            updated,
        )
        self.assertIn("| 2026-10-06 | Add PraetorScript. |", updated)

    def test_existing_release_is_idempotent(self) -> None:
        updated, changed = updater.update_page(PAGE, "unused", "v0.4.15")
        self.assertFalse(changed)
        self.assertEqual(updated, PAGE)

    def test_missing_markers_fail_closed(self) -> None:
        with self.assertRaises(updater.UpdateError):
            updater.update_page("| Release | Date | Major features and changes |", "row", "v0.5.0")


if __name__ == "__main__":
    unittest.main()
