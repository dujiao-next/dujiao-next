"""Build a three-way source update package without extracting or running upstream code."""

from concurrent.futures import ThreadPoolExecutor
from hashlib import sha256
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tarfile
import tempfile
from urllib.request import urlopen
from zipfile import ZIP_DEFLATED, ZipFile


ARCHIVE_LIMIT = 32 * 1024**2
TREE_LIMIT = 128 * 1024**2
FILE_LIMIT = 8 * 1024**2
STABLE_TAG = re.compile(r"v[0-9]+\.[0-9]+\.[0-9]+")
RUNTIME_DIRS = {"db", "data", "uploads", "logs", "backups"}
GENERATED_DIRS = {".git", "dist", "node_modules", "__pycache__"}


def load_official(tag):
    url = f"https://codeload.github.com/dujiao-next/dujiao-next/tar.gz/refs/tags/{tag}"
    with urlopen(url, timeout=18) as response:
        return read_archive(response)


def read_archive(stream):
    content = stream.read(ARCHIVE_LIMIT + 1)
    if len(content) > ARCHIVE_LIMIT:
        raise ValueError("Compressed source archive exceeds the limit")
    return content


def read_tree(payload):
    files = {}
    total = 0
    with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as archive:
        for number, member in enumerate(archive, start=1):
            if number > 15000:
                raise ValueError("Source archive contains too many entries")
            if not member.isfile():
                continue
            _, _, relative = member.name.partition("/")
            parts = relative.split("/")
            if any(part in {"", ".", ".."} for part in parts) or re.search(r"[\\:\x00-\x1f]", relative):
                raise ValueError("Invalid archive member path")
            path = PurePosixPath(relative)
            name = path.name.lower()
            if (path.parts[0] in RUNTIME_DIRS or GENERATED_DIRS.intersection(path.parts)
                    or name in {"config.yml", "config.yaml", ".env"}
                    or name.startswith(".env.")
                    or name.endswith((".key", ".pem", ".p12", ".pfx", ".db", ".sqlite", ".sqlite3"))):
                continue
            total += member.size
            if member.size > FILE_LIMIT or total > TREE_LIMIT or relative in files:
                raise ValueError("Invalid source file size or duplicate file")
            data = archive.extractfile(member).read()
            files[relative] = (data, member.mode & 0o111)
    if "go.mod" not in files or "cmd/server/main.go" not in files:
        raise ValueError("Archive does not contain the Dujiao-Next source tree")
    return files


def file_hash(entry):
    return sha256(entry[0]).hexdigest() if entry is not None else None


def merge_changed(local, base, upstream, scratch):
    if any(entry is None or b"\0" in entry[0] for entry in (local, base, upstream)):
        return None
    if local[1] != base[1] and local[1] != upstream[1] and upstream[1] != base[1]:
        return None
    paths = [scratch / name for name in ("local", "base", "upstream")]
    for path, entry in zip(paths, (local, base, upstream)):
        path.write_bytes(entry[0])
    result = subprocess.run(["diff3", "-m", *paths], capture_output=True, timeout=4)
    if result.returncode == 1:
        return None
    result.check_returncode()
    executable = upstream[1] if local[1] == base[1] else local[1]
    return result.stdout, executable


def plan_changes(base, local, upstream, scratch):
    changes = []
    conflicts = []
    counts = dict(added=0, updated=0, removed=0, merged=0)
    for path in sorted(base.keys() | upstream.keys()):
        previous = base.get(path)
        incoming = upstream.get(path)
        current = local.get(path)
        if incoming == previous or incoming == current:
            continue
        if current == previous:
            result = incoming
            action = "removed" if incoming is None else "added" if previous is None else "updated"
        else:
            result = merge_changed(current, previous, incoming, scratch)
            if result is None:
                conflicts.append(path)
                continue
            if result == current:
                continue
            action = "merged"
        change = {"path": path, "action": action, "before_sha256": file_hash(current),
                  "after_sha256": file_hash(result), "executable": bool(result and result[1])}
        changes.append((change, result))
        counts[action] += 1
    return changes, conflicts, counts


def candidate_source(local, changes, path):
    """Apply the increment to a source copy for a clean, isolated build."""
    candidate = local.copy()
    for change, entry in changes:
        if entry is None:
            del candidate[change["path"]]
        else:
            candidate[change["path"]] = entry
    with tarfile.open(path, mode="w:gz", compresslevel=5) as archive:
        for name, (data, executable) in sorted(candidate.items()):
            member = tarfile.TarInfo("source/" + name)
            member.size = len(data)
            member.mode = 0o644 | executable
            archive.addfile(member, io.BytesIO(data))


def prepare(base_tag, target_tag, snapshot, output):
    if not STABLE_TAG.fullmatch(base_tag) or not STABLE_TAG.fullmatch(target_tag):
        raise ValueError("Expected stable release tags")
    with snapshot.open("rb") as source:
        local_payload = read_archive(source)
    with ThreadPoolExecutor(max_workers=2) as downloads:
        base_payload, upstream_payload = downloads.map(load_official, (base_tag, target_tag))
    base, local, upstream = map(read_tree, (base_payload, local_payload, upstream_payload))
    with tempfile.TemporaryDirectory(prefix="dujiao-increment-", dir=output.parent) as temp:
        scratch = Path(temp)
        changes, conflicts, counts = plan_changes(base, local, upstream, scratch)
        plan = {
            "base": base_tag, "target": target_tag, "summary": counts,
            "conflicts": conflicts, "ready_for_build": not conflicts,
            "snapshot_sha256": sha256(local_payload).hexdigest(),
            "base_archive_sha256": sha256(base_payload).hexdigest(),
            "upstream_archive_sha256": sha256(upstream_payload).hexdigest(),
            "changes": [change for change, _ in changes],
        }
        package = scratch / "increment.zip"
        with ZipFile(package, "w", ZIP_DEFLATED, compresslevel=5) as archive:
            archive.writestr("plan.json", json.dumps(plan, ensure_ascii=False, indent=2))
            for change, entry in changes:
                if entry is not None:
                    archive.writestr("changes/" + change["path"], entry[0])
            if not conflicts:
                candidate = scratch / "candidate-source.tar.gz"
                candidate_source(local, changes, candidate)
                archive.write(candidate, candidate.name)
        with package.open("rb+") as file:
            os.fsync(file.fileno())
        package.replace(output)


if __name__ == "__main__":
    os.umask(0o077)
    prepare(sys.argv[1], sys.argv[2], Path(sys.argv[3]), Path(sys.argv[4]))
