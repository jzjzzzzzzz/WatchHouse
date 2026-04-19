#!/usr/bin/env python3
"""Fetch a pinned lab disk; never boot unverified downloads or cache entries."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def digest_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fetch(destination, specification, opener=urllib.request.urlopen):
    destination = Path(destination)
    if destination.exists() or destination.is_symlink():
        if destination.is_symlink() or not destination.is_file():
            raise ValueError("image cache must be a non-symlink file")
        if digest_file(destination) != specification["sha256"]:
            raise ValueError("cached image checksum mismatch; refusing reuse")
        return destination
    if not specification["url"].startswith("https://cloud-images.ubuntu.com/"):
        raise ValueError("image source must be the configured Canonical HTTPS host")
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(dir=destination.parent, prefix="image-", delete=False) as file:
            temporary = Path(file.name)
            digest = hashlib.sha256()
            size = 0
            with opener(specification["url"], timeout=30) as response:
                for chunk in iter(lambda: response.read(1024 * 1024), b""):
                    size += len(chunk)
                    if size > specification["max_download_bytes"]:
                        raise ValueError("image download exceeds pinned byte ceiling")
                    digest.update(chunk)
                    file.write(chunk)
            file.flush()
            os.fsync(file.fileno())
        if digest.hexdigest() != specification["sha256"]:
            raise ValueError("downloaded image checksum mismatch")
        os.chmod(temporary, 0o600)
        # Refuse replacing an existing destination. Hardlink publication is
        # atomic on the local filesystem and unlike replace cannot clobber it.
        os.link(temporary, destination)
        return destination
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main():
    specification = json.loads((ROOT / "lab/vm/images.json").read_text())["guest"]
    cache = ROOT / "lab/local/cache"
    cache.mkdir(parents=True, mode=0o700, exist_ok=True)
    result = fetch(cache / "noble-arm64.img", specification)
    print(json.dumps({"image": str(result.relative_to(ROOT)), "sha256": digest_file(result),
                      "build": specification["build"]}))


if __name__ == "__main__":
    main()
