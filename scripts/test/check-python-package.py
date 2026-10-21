"""Verify an isolated OffGrid SDK wheel without starting a service."""

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: check-python-package.py OFFGRID_WHEEL")
    wheel = Path(sys.argv[1]).resolve(strict=True)
    with zipfile.ZipFile(wheel) as archive:
        names = set(archive.namelist())
    if "offgrid/__init__.py" not in names:
        raise SystemExit("wheel does not contain the public offgrid package")
    forbidden = [name for name in names if "worker" in name or name.startswith("runtimes/")]
    if forbidden:
        raise SystemExit(f"service worker bundled in SDK wheel: {forbidden}")

    with tempfile.TemporaryDirectory(prefix="offgrid-sdk-wheel-") as directory:
        target = Path(directory) / "site"
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "--no-deps", "--no-index", "--target", str(target), str(wheel)],
            check=True,
        )
        env = os.environ.copy()
        env["PYTHONPATH"] = str(target)
        subprocess.run(
            [sys.executable, "-c", "import importlib.metadata as m; import offgrid; "
             "assert m.version('offgrid') == offgrid.__version__; "
             "assert hasattr(offgrid, 'Client') and hasattr(offgrid, 'Agent')"],
            cwd=directory,
            env=env,
            check=True,
        )
    print(f"SDK wheel verified: {wheel.name}")


if __name__ == "__main__":
    main()
