"""验证 ZIP 结构、权限与 staging 边界；不启动任何服务。"""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import zipfile
from pack_bundle import pack


class BundleTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.stage = self.base / 'stage'
        (self.stage / 'dist').mkdir(parents=True)
        (self.stage / 'dist/index.html').write_text('<html>test</html>')
        self.manifest = {'name': 'demo', 'version': '1.0.0', 'type': 'static', 'static': {'directory': 'dist'}}
        self.write_manifest()
        self.output = self.base / 'bundle.zip'

    def write_manifest(self):
        (self.stage / 'upy.yaml').write_text(json.dumps(self.manifest))

    def test_static_root_and_checksum(self):
        pack(self.stage, self.output)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual(set(archive.namelist()), {'upy.yaml', 'dist/index.html'})
        self.assertEqual(Path(str(self.output)+'.sha256').read_text().split()[0], hashlib.sha256(self.output.read_bytes()).hexdigest())

    def test_system_executable_permissions(self):
        self.manifest = {'name': 'demo', 'version': '1.0.0', 'type': 'system', 'target': {'os': 'linux', 'arch': 'arm64'}, 'system': {'command': ['./bin/server']}}
        self.write_manifest()
        (self.stage / 'bin').mkdir()
        binary = self.stage / 'bin/server'
        binary.write_bytes(b'fixture executable; not ELF')
        binary.chmod(0o644)
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)
        binary.chmod(0o755)
        pack(self.stage, self.output)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual((archive.getinfo('bin/server').external_attr >> 16) & 0o777, 0o755)

    def test_path_escape_rejected(self):
        self.manifest['static']['directory'] = '../outside'
        self.write_manifest()
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)

    def test_symlink_rejected(self):
        (self.stage / 'dist/secret').symlink_to('/etc/passwd')
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)

    def test_secrets_rejected(self):
        (self.stage / '.env').write_text('SECRET=fake')
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)

    def test_output_inside_stage_rejected(self):
        with self.assertRaises(ValueError):
            pack(self.stage, self.stage / 'bundle.zip')

    def test_missing_entry_rejected(self):
        (self.stage / 'dist/index.html').unlink()
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)

    def test_repeat_immutable(self):
        pack(self.stage, self.output)
        with self.assertRaises(ValueError):
            pack(self.stage, self.output)


if __name__ == '__main__':
    unittest.main()
