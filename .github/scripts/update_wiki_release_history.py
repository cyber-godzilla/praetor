#!/usr/bin/env python3
"""Insert a published Praetor release into the TEC Wiki release table."""

from __future__ import annotations

import argparse
import json
import re
import textwrap
from datetime import datetime
from pathlib import Path


START_MARKER = "<!-- praetor-release-history:start -->"
END_MARKER = "<!-- praetor-release-history:end -->"
TABLE_HEADER = "| Release | Date | Major features and changes |"
TABLE_SEPARATOR = "| --- | --- | --- |"
TAG_PATTERN = re.compile(r"^v\d+\.\d+\.\d+(?:[-.][0-9A-Za-z.-]+)?$")
EXPLICIT_SUMMARY_PATTERN = re.compile(
    r"<!--\s*wiki-release-summary:\s*(.*?)\s*-->", re.IGNORECASE | re.DOTALL
)
HEADING_PATTERN = re.compile(r"^##\s+(.+?)\s*$", re.MULTILINE)
LINK_PATTERN = re.compile(r"\[([^]]+)]\([^)]+\)")
ATTRIBUTION_PATTERN = re.compile(r"\s+by\s+@\S+\s+in\s+https?://\S+\s*$", re.IGNORECASE)
CONVENTIONAL_PREFIX_PATTERN = re.compile(
    r"^(?:feat|fix|perf|docs|build|ci|test|refactor|style|chore)(?:\([^)]*\))?!?:\s*",
    re.IGNORECASE,
)


class UpdateError(ValueError):
    """Raised when release metadata or the managed wiki section is invalid."""


def _heading_section(body: str, wanted: str) -> str:
    matches = list(HEADING_PATTERN.finditer(body))
    for index, match in enumerate(matches):
        if match.group(1).strip().casefold() != wanted.casefold():
            continue
        end = matches[index + 1].start() if index + 1 < len(matches) else len(body)
        return body[match.end() : end].strip()
    return ""


def _clean_summary_item(value: str) -> str:
    value = LINK_PATTERN.sub(r"\1", value)
    value = ATTRIBUTION_PATTERN.sub("", value)
    value = re.sub(r"https?://\S+", "", value)
    value = re.sub(r"[*_~]", "", value)
    value = CONVENTIONAL_PREFIX_PATTERN.sub("", value.strip())
    value = re.sub(r"\s+", " ", value).strip(" -.;")
    if not value:
        return ""
    value = value[0].upper() + value[1:]
    if value[-1] not in ".!?":
        value += "."
    return value


def _bullet_items(section: str) -> list[str]:
    items: list[str] = []
    for line in section.splitlines():
        match = re.match(r"^\s*[-*]\s+(.+?)\s*$", line)
        if not match:
            continue
        item = _clean_summary_item(match.group(1))
        if item and not item.casefold().startswith("full changelog"):
            items.append(item)
    return items


def _commit_items(subjects: list[str]) -> list[str]:
    ranked: list[tuple[int, int, str]] = []
    for index, subject in enumerate(subjects):
        stripped = subject.strip()
        lowered = stripped.casefold()
        if not stripped or lowered.startswith("merge "):
            continue
        if re.match(r"^chore\(release\):", stripped, re.IGNORECASE):
            continue
        if re.search(r"\b(?:bump|release)\s+v?\d+\.\d+\.\d+\b", lowered):
            continue
        if re.match(r"^(?:feat:\s*)?prepare\s+praetor\s+v?\d+\.\d+\.\d+", lowered):
            continue
        if re.match(r"^feat(?:\(|:)", stripped, re.IGNORECASE):
            priority = 0
        elif re.match(r"^(?:fix|perf)(?:\(|:)", stripped, re.IGNORECASE):
            priority = 1
        elif re.match(r"^(?:chore|ci|test|style|refactor)(?:\(|:)", stripped, re.IGNORECASE):
            continue
        else:
            priority = 2
        item = _clean_summary_item(stripped)
        if item:
            ranked.append((priority, index, item))
    ranked.sort(key=lambda item: (item[0], item[1]))
    return [item for _, _, item in ranked]


def release_summary(body: str, commit_subjects: list[str]) -> str:
    explicit = EXPLICIT_SUMMARY_PATTERN.search(body)
    if explicit:
        items = [_clean_summary_item(explicit.group(1))]
    else:
        wiki_section = _heading_section(body, "Wiki release summary")
        if wiki_section:
            items = [_clean_summary_item(wiki_section)]
        else:
            items = _bullet_items(_heading_section(body, "What's Changed"))
            if not items:
                items = _commit_items(commit_subjects)

    unique: list[str] = []
    for item in items:
        if item and item not in unique:
            unique.append(item)
        if len(unique) == 3:
            break
    if not unique:
        raise UpdateError(
            "release has no usable summary; add "
            "<!-- wiki-release-summary: Major user-facing changes. --> to its notes"
        )

    summary = " ".join(unique)
    if len(summary) > 480:
        summary = textwrap.shorten(summary, width=480, placeholder="…")
    return summary.replace("|", r"\|")


def release_row(release: dict[str, object], commit_subjects: list[str]) -> str:
    tag = str(release.get("tag_name", ""))
    if not TAG_PATTERN.fullmatch(tag):
        raise UpdateError(f"invalid release tag: {tag!r}")

    url = str(release.get("html_url", ""))
    expected_url = f"https://github.com/cyber-godzilla/praetor/releases/tag/{tag}"
    if url != expected_url:
        raise UpdateError(f"unexpected release URL: {url!r}")

    published_at = str(release.get("published_at", ""))
    try:
        date = datetime.fromisoformat(published_at.replace("Z", "+00:00")).date().isoformat()
    except ValueError as error:
        raise UpdateError(f"invalid publication date: {published_at!r}") from error

    summary = release_summary(str(release.get("body") or ""), commit_subjects)
    return f"| [{tag}]({url}) | {date} | {summary} |"


def update_page(content: str, row: str, tag: str) -> tuple[str, bool]:
    if content.count(START_MARKER) != 1 or content.count(END_MARKER) != 1:
        raise UpdateError("release-history markers must each occur exactly once")

    start = content.index(START_MARKER)
    end = content.index(END_MARKER, start)
    managed = content[start:end]
    if re.search(rf"^\| \[{re.escape(tag)}]", managed, re.MULTILINE):
        return content, False

    lines = content.splitlines(keepends=True)
    start_line = next(index for index, line in enumerate(lines) if START_MARKER in line)
    end_line = next(index for index, line in enumerate(lines[start_line + 1 :], start_line + 1) if END_MARKER in line)
    header_line = next(
        (index for index in range(start_line + 1, end_line) if lines[index].rstrip("\r\n") == TABLE_HEADER),
        None,
    )
    if header_line is None or header_line + 1 >= end_line:
        raise UpdateError("managed release-history table header is missing")
    if lines[header_line + 1].rstrip("\r\n") != TABLE_SEPARATOR:
        raise UpdateError("managed release-history table separator is missing")

    newline = "\r\n" if lines[header_line + 1].endswith("\r\n") else "\n"
    lines.insert(header_line + 2, row + newline)
    return "".join(lines), True


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--page", type=Path, required=True)
    parser.add_argument("--release-json", type=Path, required=True)
    parser.add_argument("--commit-subjects", type=Path)
    args = parser.parse_args()

    release = json.loads(args.release_json.read_text(encoding="utf-8"))
    subjects = (
        args.commit_subjects.read_text(encoding="utf-8").splitlines()
        if args.commit_subjects
        else []
    )
    row = release_row(release, subjects)
    original = args.page.read_text(encoding="utf-8")
    updated, changed = update_page(original, row, str(release["tag_name"]))
    if changed:
        args.page.write_text(updated, encoding="utf-8")
        print(f"added {release['tag_name']} to {args.page}")
    else:
        print(f"{release['tag_name']} is already present in {args.page}")


if __name__ == "__main__":
    main()
