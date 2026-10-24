"""Independent file-effect checks for the disposable external-agent harness."""
import hashlib
import os
from pathlib import Path
import stat


def workspace_snapshot(root):
    """Fingerprint entries without following agent-created symbolic links."""
    root = Path(root)
    entries = {}
    for directory, dirs, files in os.walk(root, followlinks=False):
        for name in sorted(dirs + files):
            path = Path(directory) / name
            key = path.relative_to(root).as_posix()
            info = path.lstat()
            if stat.S_ISLNK(info.st_mode):
                entries[key] = ("symlink", os.readlink(path))
            elif stat.S_ISDIR(info.st_mode):
                entries[key] = ("directory",)
            elif stat.S_ISREG(info.st_mode):
                if info.st_size > 8 << 20:
                    raise ValueError("Disposable workspace contains an oversized file")
                entries[key] = ("file", hashlib.sha256(path.read_bytes()).hexdigest())
            else:
                entries[key] = ("special", stat.S_IFMT(info.st_mode))
    return entries


def changed_paths(before, after):
    return sorted(path for path in before.keys() | after.keys()
                  if before.get(path) != after.get(path))


def verify_copy(root, expected, before):
    root = Path(root)

    def exact_file(name):
        path = root / name
        try:
            info = path.lstat()
            return (stat.S_ISREG(info.st_mode) and info.st_size == len(expected)
                    and path.read_bytes() == expected)
        except FileNotFoundError:
            return False

    unexpected = [path for path in changed_paths(before, workspace_snapshot(root))
                  if path != "copy.txt"]
    source_preserved = exact_file("source.txt")
    destination_exact = exact_file("copy.txt")
    return {"stage": "independent-file-readback",
            "passed": source_preserved and destination_exact and not unexpected,
            "source_preserved": source_preserved, "destination_exact": destination_exact,
            "unexpected_changes": unexpected}
