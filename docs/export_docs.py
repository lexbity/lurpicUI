#!/usr/bin/env python3
"""Convert Markdown documentation into Hugo-compatible content.

Reads every ``*.md`` file under a source directory (default ``./docs``),
categorizes it by filename keywords, and writes it under ``content/docs/``
with generated Hugo front matter.

Categorization (first matching rule wins, matched case-insensitively against
the filename stem):

* ``readme`` / ``intro`` / ``overview`` / ``install`` / ``setup`` /
  ``configure``                      -> ``getting-started/``
* ``usage`` / ``guide`` / ``tutorial`` -> ``guides/``
* ``api`` / ``reference`` / ``docs``   -> ``reference/``
* a file matching no keyword stays at the root of the output
* a file inside a subdirectory of the source keeps that subdirectory path: a
  dedicated "book" folder keeps its custom name instead of being categorized

Front matter:

* ``title``   -- ``"[Project Name] - [Extracted Title]"``; the title is the
  first ``# `` heading of the document, falling back to the filename.
* ``weight``  -- ``10 + (category_index * 5)`` where ``category_index`` is the
  0-based position of the file among the files assigned to its category
  (sorted by filename), so every page in a section gets a distinct weight and
  Hugo orders them deterministically.
* ``type``    -- ``docs``
* ``draft``   -- ``false``
* ``lastmod`` -- the source file's mtime in UTC as
  ``YYYY-MM-DDTHH:MM:SS+00:00``

Existing front matter in a source file is stripped and regenerated; the body
is copied through unchanged.

Project name: ``--project-name``, defaulting to the name of the directory that
contains the source directory (e.g. the repository name for ``./docs``).

Usage::

    python3 export_docs.py [--source DIR] [--output DIR]
                           [--project-name NAME] [--dry-run]
"""

from __future__ import annotations

import argparse
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

# Category keyword rules, evaluated in order; first match wins.
CATEGORY_RULES: list[tuple[str, tuple[str, ...]]] = [
    ("getting-started", ("readme", "intro", "overview", "install", "setup", "configure")),
    ("guides", ("usage", "guide", "tutorial")),
    ("reference", ("api", "reference", "docs")),
]

FRONT_MATTER_DELIM = re.compile(r"^---\s*$")


def slugify(stem: str) -> str:
    """Turn a filename stem into a Hugo-friendly file name."""
    slug = re.sub(r"\s+", "-", stem.strip())
    slug = re.sub(r"[^A-Za-z0-9._-]", "", slug)
    return slug or "untitled"


def strip_front_matter(text: str) -> tuple[dict[str, str] | None, str]:
    """Split an existing ``---``-delimited front matter block off the top."""
    lines = text.split("\n")
    if not lines or not FRONT_MATTER_DELIM.match(lines[0]):
        return None, text
    for i in range(1, len(lines)):
        if FRONT_MATTER_DELIM.match(lines[i]) or lines[i].strip() == "...":
            meta_lines = lines[1:i]
            meta: dict[str, str] = {}
            for line in meta_lines:
                key, sep, value = line.partition(":")
                if sep:
                    meta[key.strip()] = value.strip()
            return meta, "\n".join(lines[i + 1 :]).lstrip("\n")
    # Opening --- without a closing one: treat the whole file as content.
    return None, text


def extract_title(body: str, stem: str) -> str:
    """First ``# `` heading in the body, else a title-cased filename."""
    for line in body.split("\n"):
        match = re.match(r"^#\s+(.+?)\s*$", line)
        if match:
            return match.group(1).strip().lstrip("#").strip()
    words = re.split(r"[-_\s]+", stem.strip())
    return " ".join(w.capitalize() for w in words if w) or stem


def categorize(rel_path: Path) -> str:
    """Output subdirectory for a source file, relative to the output root.

    Files in the source root are categorized by filename keywords; files in a
    subdirectory keep their subdirectory path (book folders keep their name).
    """
    if len(rel_path.parts) > 1:
        return "/".join(rel_path.parts[:-1])
    stem = rel_path.stem.lower()
    for category, keywords in CATEGORY_RULES:
        if any(keyword in stem for keyword in keywords):
            return category
    return "."


def yaml_quote(value: str) -> str:
    """Quote a string for YAML, escaping backslash and double quote."""
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def build_front_matter(title: str, weight: int, lastmod: str) -> str:
    return (
        "---\n"
        f"title: {yaml_quote(title)}\n"
        f"weight: {weight}\n"
        "type: docs\n"
        "draft: false\n"
        f"lastmod: {lastmod}\n"
        "---\n\n"
    )


def collect_source_files(source: Path, output: Path) -> list[Path]:
    """All ``*.md`` files under ``source``, skipping hidden dirs and ``output``."""
    resolved_output = output.resolve()
    files: list[Path] = []
    for path in sorted(source.rglob("*.md")):
        if not path.is_file():
            continue
        if any(part.startswith(".") for part in path.relative_to(source).parts):
            continue
        if resolved_output == path.resolve() or resolved_output in path.resolve().parents:
            continue
        files.append(path)
    return files


def convert(source: Path, output: Path, project_name: str, dry_run: bool) -> int:
    files = collect_source_files(source, output)
    if not files:
        print(f"No Markdown files found under {source}", file=sys.stderr)
        return 1

    # Assign weights: index within each category, files sorted by path.
    category_members: dict[str, list[Path]] = {}
    for path in files:
        category_members.setdefault(categorize(path.relative_to(source)), []).append(path)

    written: list[tuple[Path, Path]] = []
    for category in sorted(category_members):
        for index, path in enumerate(sorted(category_members[category])):
            raw = path.read_text(encoding="utf-8", errors="replace")
            _, body = strip_front_matter(raw)
            title = f"{project_name} - {extract_title(body, path.stem)}"
            lastmod = datetime.fromtimestamp(path.stat().st_mtime, tz=timezone.utc).strftime(
                "%Y-%m-%dT%H:%M:%S+00:00"
            )
            weight = 10 + index * 5

            rel = path.relative_to(source)
            dest_dir = output if categorize(rel) == "." else output / categorize(rel)
            dest = dest_dir / f"{slugify(rel.stem)}.md"
            front_matter = build_front_matter(title, weight, lastmod)

            if dry_run:
                print(f"{path} -> {dest}  (weight {weight})")
            else:
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_text(front_matter + body, encoding="utf-8")
            written.append((path, dest))

    verb = "Would write" if dry_run else "Wrote"
    print(f"\n{verb} {len(written)} file(s) to {output}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--source", type=Path, default=Path("docs"),
                        help="directory containing the source .md files (default: ./docs)")
    parser.add_argument("--output", type=Path, default=Path("content/docs"),
                        help="Hugo content output directory (default: ./content/docs)")
    parser.add_argument("--project-name", default=None,
                        help="project name for titles (default: name of the source's parent directory)")
    parser.add_argument("--dry-run", action="store_true",
                        help="print the conversion plan without writing files")
    args = parser.parse_args(argv)

    source = args.source.resolve()
    if not source.is_dir():
        print(f"Source directory not found: {source}", file=sys.stderr)
        return 1
    output = args.output.resolve()
    if output == source or source in output.parents:
        print(f"Output directory {output} must not be inside source {source}", file=sys.stderr)
        return 1

    project_name = args.project_name or source.parent.name or source.name
    return convert(source, output, project_name, args.dry_run)


if __name__ == "__main__":
    sys.exit(main())
