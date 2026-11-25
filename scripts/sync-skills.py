#!/usr/bin/env python3
import argparse
import shutil
import tempfile
from pathlib import Path


def require_disjoint(source: Path, destination: Path) -> None:
    if source == destination or source in destination.parents or destination in source.parents:
        raise ValueError(f"source and destination must not overlap: {source} and {destination}")


def copy_catalog(source: Path, staging: Path) -> int:
    skills = []
    for path in sorted(source.iterdir()):
        if path.is_symlink():
            raise ValueError(f"skill source contains a symlink: {path}")
        if not path.is_dir():
            raise ValueError(f"unexpected file at skills root: {path}")
        if not (path / "SKILL.md").is_file():
            raise ValueError(f"missing SKILL.md: {path}")
        for child in path.rglob("*"):
            if child.is_symlink():
                raise ValueError(f"skill contains a symlink: {child}")
        skills.append(path)
    if not skills:
        raise ValueError(f"skills source is empty: {source}")

    for skill in skills:
        shutil.copytree(skill, staging / skill.name)
    return len(skills)


def replace_catalog(staging: Path, destination: Path) -> None:
    backup_parent = None
    backup = None
    try:
        if destination.exists() or destination.is_symlink():
            backup_parent = Path(tempfile.mkdtemp(prefix=f".{destination.name}.backup-", dir=destination.parent))
            backup = backup_parent / "catalog"
            destination.replace(backup)
        staging.replace(destination)
    except Exception:
        if backup is not None and backup.exists():
            backup.replace(destination)
        raise
    finally:
        if staging.exists():
            shutil.rmtree(staging)
        if backup_parent is not None and backup is not None and not backup.exists():
            shutil.rmtree(backup_parent)


def sync_skills(source: Path, destination: Path) -> int:
    if source.is_symlink() or not source.is_dir():
        raise ValueError(f"skills source is not a directory: {source}")
    source = source.resolve()
    destination = destination.resolve()
    require_disjoint(source, destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix=f".{destination.name}.staging-", dir=destination.parent))
    require_disjoint(source, staging)
    try:
        count = copy_catalog(source, staging)
        replace_catalog(staging, destination)
        return count
    finally:
        if staging.exists():
            shutil.rmtree(staging)


def main() -> None:
    parser = argparse.ArgumentParser(description="Sync MCP skills from scrapfly/skills")
    parser.add_argument("--skills", type=Path, required=True, help="path to the golden skills directory")
    parser.add_argument("--out", type=Path, required=True, help="destination embedded by the MCP server")
    args = parser.parse_args()
    count = sync_skills(args.skills, args.out)
    print(f"Synchronized {count} skills into {args.out}")


if __name__ == "__main__":
    main()
