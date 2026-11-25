import importlib.util
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).parents[1] / "sync-skills.py"
SPEC = importlib.util.spec_from_file_location("sync_skills", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class SyncSkillsTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.destination = self.root / "embedded"
        (self.source / "alpha" / "references").mkdir(parents=True)
        (self.source / "beta").mkdir()
        (self.source / "alpha" / "SKILL.md").write_text("alpha\n")
        (self.source / "alpha" / "references" / "guide.md").write_text("guide\n")
        (self.source / "beta" / "SKILL.md").write_text("beta\n")

    def tearDown(self):
        self.temporary.cleanup()

    def create_existing_catalog(self):
        (self.destination / "existing").mkdir(parents=True)
        (self.destination / "existing" / "SKILL.md").write_text("existing\n")

    def assert_existing_catalog_untouched(self):
        self.assertEqual((self.destination / "existing" / "SKILL.md").read_text(), "existing\n")
        self.assertEqual([path.name for path in self.destination.iterdir()], ["existing"])

    def test_sync_copies_every_file_and_removes_stale_content(self):
        (self.destination / "stale").mkdir(parents=True)
        (self.destination / "stale" / "SKILL.md").write_text("stale\n")

        count = MODULE.sync_skills(self.source, self.destination)

        self.assertEqual(count, 2)
        self.assertFalse((self.destination / "stale").exists())
        for path in self.source.rglob("*"):
            if path.is_file():
                relative = path.relative_to(self.source)
                self.assertEqual((self.destination / relative).read_bytes(), path.read_bytes())

    def test_sync_refuses_a_nested_symlink(self):
        target = self.source / "alpha" / "references" / "guide.md"
        link = self.source / "alpha" / "link.md"
        try:
            link.symlink_to(target.name)
        except (OSError, NotImplementedError):
            self.skipTest("symlinks are unavailable")

        with self.assertRaisesRegex(ValueError, "contains a symlink"):
            MODULE.sync_skills(self.source, self.destination)

    def test_sync_refuses_a_top_level_skill_symlink(self):
        outside = self.root / "outside"
        outside.mkdir()
        (outside / "SKILL.md").write_text("outside\n")
        shutil.rmtree(self.source / "alpha")
        try:
            (self.source / "alpha").symlink_to(outside, target_is_directory=True)
        except (OSError, NotImplementedError):
            self.skipTest("symlinks are unavailable")
        self.create_existing_catalog()

        with self.assertRaisesRegex(ValueError, "contains a symlink"):
            MODULE.sync_skills(self.source, self.destination)

        self.assert_existing_catalog_untouched()

    def test_sync_refuses_a_directory_without_skill_markdown(self):
        (self.source / "beta" / "SKILL.md").unlink()

        with self.assertRaisesRegex(ValueError, "missing SKILL.md"):
            MODULE.sync_skills(self.source, self.destination)

    def test_sync_refuses_missing_and_empty_sources_without_replacing_the_catalog(self):
        self.create_existing_catalog()
        for source in [self.root / "missing", self.root / "empty"]:
            with self.subTest(source=source.name):
                if source.name == "empty":
                    source.mkdir()
                with self.assertRaises(ValueError):
                    MODULE.sync_skills(source, self.destination)
                self.assert_existing_catalog_untouched()

    def test_sync_refuses_overlapping_source_and_destination(self):
        destination = self.source / "alpha" / "embedded"
        before = (self.source / "alpha" / "SKILL.md").read_bytes()

        with self.assertRaisesRegex(ValueError, "must not overlap"):
            MODULE.sync_skills(self.source, destination)

        self.assertEqual((self.source / "alpha" / "SKILL.md").read_bytes(), before)
        self.assertFalse(destination.exists())

    def test_failed_final_rename_restores_the_existing_catalog(self):
        self.create_existing_catalog()
        original_replace = Path.replace
        resolved_destination = self.destination.resolve()

        def fail_staging_replace(source, target):
            if source.name.startswith(".embedded.staging-") and target == resolved_destination:
                raise OSError("injected rename failure")
            return original_replace(source, target)

        with mock.patch.object(Path, "replace", fail_staging_replace):
            with self.assertRaisesRegex(OSError, "injected rename failure"):
                MODULE.sync_skills(self.source, self.destination)

        self.assert_existing_catalog_untouched()
        self.assertEqual(list(self.root.glob(".embedded.backup-*")), [])
        self.assertEqual(list(self.root.glob(".embedded.staging-*")), [])

    def test_failed_sync_leaves_the_existing_catalog_untouched(self):
        self.create_existing_catalog()
        (self.source / "beta" / "SKILL.md").unlink()

        with self.assertRaises(ValueError):
            MODULE.sync_skills(self.source, self.destination)

        self.assert_existing_catalog_untouched()


if __name__ == "__main__":
    unittest.main()
