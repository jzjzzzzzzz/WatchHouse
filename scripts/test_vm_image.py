import hashlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("vm_image", Path(__file__).with_name("vm_image.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ImageTests(unittest.TestCase):
    def test_verified_publication_and_cache(self):
        body = b"verified-synthetic-image"
        specification = dict(url="https://cloud-images.ubuntu.com/test.img", sha256=hashlib.sha256(body).hexdigest(), max_download_bytes=100)
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "disk.img"
            module.fetch(path, specification, lambda *a, **kw: io.BytesIO(body))
            self.assertEqual(path.read_bytes(), body)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            module.fetch(path, specification, lambda *a, **kw: self.fail("verified cache redownloaded"))

    def test_bad_digest_and_size_leave_no_image(self):
        for body, ceiling in ((b"bad", 100), (b"too-big", 2)):
            specification = dict(url="https://cloud-images.ubuntu.com/test.img", sha256="0" * 64, max_download_bytes=ceiling)
            with tempfile.TemporaryDirectory() as temporary:
                path = Path(temporary) / "disk.img"
                with self.assertRaises(ValueError):
                    module.fetch(path, specification, lambda *a, **kw: io.BytesIO(body))
                self.assertFalse(path.exists())
                self.assertEqual(list(Path(temporary).iterdir()), [])

    def test_symlink_and_modified_cache_rejected(self):
        specification = dict(url="https://cloud-images.ubuntu.com/test.img", sha256="0" * 64, max_download_bytes=100)
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "disk.img"
            path.write_bytes(b"modified")
            with self.assertRaises(ValueError):
                module.fetch(path, specification)
            path.unlink()
            path.symlink_to(Path(temporary) / "missing")
            with self.assertRaises(ValueError):
                module.fetch(path, specification)
