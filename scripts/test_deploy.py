"""Exercise make deploy with real rsync and stubbed Go/Supervisor commands."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class DeployTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.source = root / "checkout"
        self.app = root / "production"
        self.commands = root / "commands"
        self.log = root / "commands.log"
        for directory in (self.source / "scripts", self.app / "bin", self.commands):
            directory.mkdir(parents=True)
        repo = Path(__file__).resolve().parent.parent
        for name in ("Makefile", "scripts/deploy.sh"):
            shutil.copy(repo / name, self.source / name)
        (self.source / ".env").write_text("DATABASE_URL=development\n")
        (self.app / ".env").write_text("DATABASE_URL=production\n")
        (self.app / ".env").chmod(0o600)
        (self.app / "bin/picture-this").write_text("old binary")
        self.stub("supervisorctl", 'echo "supervisor $*" >> "$DEPLOY_TEST_LOG"\n')
        self.stub("go", '''echo "go $*" >> "$DEPLOY_TEST_LOG"
if [ "$1" = tool ]; then
  echo generated > generated.go
elif [ "$1" = build ]; then
  [ "${DEPLOY_TEST_FAIL:-}" != build ] || exit 1
  if [ "$3" = bin/migrate ]; then
    cat > "$3" <<'MIGRATE'
#!/usr/bin/env bash
set -eu
[ -z "${DATABASE_URL:-}" ]
test "$(cat .env)" = DATABASE_URL=production
echo migrate >> "$DEPLOY_TEST_LOG"
[ "${DEPLOY_TEST_FAIL:-}" != migrate ]
MIGRATE
    chmod +x "$3"
  else
    echo 'new binary' > "$3"
  fi
fi
''')
        self.env = dict(os.environ, PATH=f"{self.commands}:{os.environ['PATH']}",
                        DEPLOY_TEST_LOG=str(self.log), DATABASE_URL="inherited-development")

    def stub(self, name, body):
        path = self.commands / name
        path.write_text("#!/usr/bin/env bash\nset -eu\n" + body)
        path.chmod(0o755)

    def deploy(self, app=None, fail=""):
        result = subprocess.run(
            ["make", "deploy", f"APP_DIR={app or self.app}"], cwd=self.source,
            env=dict(self.env, DEPLOY_TEST_FAIL=fail), capture_output=True, text=True)
        return result

    def test_deploy_preserves_production_files_and_excludes_local_artifacts(self):
        for base in (self.source, self.app):
            (base / "static/audio").mkdir(parents=True)
            (base / "static/sounds").mkdir(parents=True)
        (self.app / "static/audio/server.mp3").write_text("server audio")
        (self.app / "static/sounds/server.wav").write_text("server sound")
        (self.source / "static/audio/new.mp3").write_text("new audio")
        (self.app / "obsolete.go").write_text("obsolete")
        for name in (".git", ".gocache", "node_modules", ".venv-joke-audio"):
            (self.source / name).mkdir()
            (self.source / name / "local").write_text("local")
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.app / ".env").read_text(), "DATABASE_URL=production\n")
        self.assertEqual((self.app / ".env").stat().st_mode & 0o777, 0o600)
        self.assertEqual((self.app / "bin/picture-this").read_text(), "new binary\n")
        self.assertTrue((self.app / "generated.go").exists())
        for name in ("server.mp3", "new.mp3"):
            self.assertTrue((self.app / "static/audio" / name).exists())
        self.assertTrue((self.app / "static/sounds/server.wav").exists())
        for name in ("obsolete.go", ".git", ".gocache", "node_modules", ".venv-joke-audio"):
            self.assertFalse((self.app / name).exists())
        log = self.log.read_text()
        self.assertLess(log.index("go tool templ generate"), log.index("go build"))
        self.assertLess(log.index("migrate\n"), log.index("supervisor restart"))

    def test_failures_leave_live_binary_and_assets_untouched(self):
        for failure in ("build", "migrate"):
            with self.subTest(failure=failure):
                self.log.write_text("")
                (self.app / "obsolete.go").write_text("old source")
                result = self.deploy(fail=failure)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual((self.app / "bin/picture-this").read_text(), "old binary")
                self.assertTrue((self.app / "obsolete.go").exists())
                self.assertNotIn("supervisor restart", self.log.read_text())

    def test_in_place_deploy(self):
        (self.source / ".env").write_text("DATABASE_URL=production\n")
        result = self.deploy(app=self.source)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.source / "scripts/deploy.sh").exists())
        self.assertEqual((self.source / "bin/picture-this").read_text(), "new binary\n")

    def test_unsafe_or_unprovisioned_destination(self):
        nested = self.source / "production"
        nested.mkdir()
        for app in ("/", nested, self.source.parent, self.source.parent / "missing"):
            with self.subTest(app=app):
                result = self.deploy(app=app)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.log.exists())


if __name__ == "__main__":
    unittest.main()
