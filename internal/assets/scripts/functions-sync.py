"""Mirror a project's Function resources into a shared volume."""

import json
import math
import os
from pathlib import Path, PurePosixPath
import ssl
import tempfile
import time
import urllib.parse
import urllib.request


def relative_path(value):
    if not isinstance(value, str) or not value or "\\" in value or "\x00" in value:
        raise ValueError("Invalid function path")
    path = PurePosixPath(value)
    if path.is_absolute() or any(part in ("", ".", "..") for part in value.split("/")):
        raise ValueError("Function path must stay inside its directory")
    return path


def snapshot(items, project, required_file=""):
    files = {}
    names = set()
    for item in items:
        spec = item["spec"]
        if spec["projectRef"]["name"] != project or item.get("metadata", {}).get("deletionTimestamp"):
            continue
        name = relative_path(spec["functionName"])
        if len(name.parts) != 1 or str(name) in names:
            raise ValueError("Invalid or duplicate logical function name")
        names.add(str(name))
        source = spec["source"]
        if not isinstance(source, dict) or not source:
            raise ValueError("Function source must contain files")
        for filename, content in source.items():
            if not isinstance(content, str):
                raise ValueError("Function source must contain text")
            files[str(name / relative_path(filename))] = content.encode("utf-8")
    for filename in files:
        if any(str(parent) in files for parent in PurePosixPath(filename).parents):
            raise ValueError("Function files conflict with directories")
    if required_file and str(relative_path(required_file)) not in files:
        raise ValueError("Waiting for required function file")
    return files


def fetch_items(api_url, token_file, ca_file, timeout):
    # Read the projected token on every request so rotation needs no restart.
    context = ssl.create_default_context(cafile=ca_file) if api_url.startswith("https:") else None
    items = []
    continuation = ""
    while True:
        query = urllib.parse.urlencode({"limit": 500, "continue": continuation})
        request = urllib.request.Request(
            api_url + "?" + query,
            headers={"Authorization": "Bearer " + Path(token_file).read_text().strip()},
        )
        with urllib.request.urlopen(request, context=context, timeout=timeout) as response:
            page = json.load(response)
        if not isinstance(page["items"], list):
            raise ValueError("Invalid Function list")
        items.extend(page["items"])
        continuation = page.get("metadata", {}).get("continue", "")
        if not continuation:
            return items


def sync_files(root, files):
    root = Path(root)
    if not root.is_absolute() or root == Path("/") or root.is_symlink():
        raise ValueError("SYNC_DIR must be an absolute volume directory")
    root.mkdir(parents=True, exist_ok=True)
    if root.resolve() != root:
        raise ValueError("SYNC_DIR must not traverse symlinks")
    existing_files = set()
    existing_dirs = []
    for directory, dirs, filenames in os.walk(root, followlinks=False):
        for name in dirs + filenames:
            if (Path(directory) / name).is_symlink():
                raise ValueError("Sync directory must not contain symlinks")
        existing_dirs.extend(Path(directory) / name for name in dirs)
        existing_files.update(str((Path(directory) / name).relative_to(root)) for name in filenames)

    # The full API snapshot and local paths have been validated before any deletion.
    removed = existing_files - files.keys()
    for filename in removed:
        (root / filename).unlink()
    for directory in sorted(existing_dirs, key=lambda p: len(p.parts), reverse=True):
        if not any(directory.iterdir()):
            directory.rmdir()

    changed = 0
    for filename, content in files.items():
        target = root / filename
        if target.is_file() and target.read_bytes() == content:
            continue
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        # Temporary file on the same filesystem, followed by atomic replacement.
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(dir=target.parent, prefix=".functions-sync-", delete=False) as stream:
                temporary = Path(stream.name)
                stream.write(content)
            temporary.chmod(0o644)
            os.replace(temporary, target)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)
        changed += 1
    if changed or removed:
        print(f"Synced Function files: updated={changed} removed={len(removed)}", flush=True)


def positive_seconds(name, default):
    value = float(os.environ.get(name, default))
    if not math.isfinite(value) or value <= 0:
        raise ValueError(name + " must be positive and finite")
    return value


def main():
    interval = positive_seconds("SYNC_INTERVAL_SECONDS", "10")
    timeout = positive_seconds("SYNC_REQUEST_TIMEOUT_SECONDS", "10")
    project = os.environ["PROJECT_NAME"]
    namespace = urllib.parse.quote(os.environ["POD_NAMESPACE"], safe="")
    api_url = os.environ.get(
        "SYNC_API_URL",
        f"https://kubernetes.default.svc/apis/core.supabase.io/v1alpha1/namespaces/{namespace}/functions",
    )
    credentials = "/var/run/secrets/functions-sync"
    token_file = os.environ.get("SYNC_TOKEN_FILE", credentials + "/token")
    ca_file = os.environ.get("SYNC_CA_FILE", credentials + "/ca.crt")
    root = os.environ.get("SYNC_DIR", "/functions")
    required_file = os.environ.get("SYNC_REQUIRED_FILE", "")
    once = os.environ.get("SYNC_ONCE", "false").lower() == "true"
    while True:
        try:
            files = snapshot(fetch_items(api_url, token_file, ca_file, timeout), project, required_file)
            sync_files(root, files)
            if once:
                return
        except (OSError, ValueError, KeyError, TypeError) as error:
            # Do not log API responses, tokens, or function contents.
            print(f"Could not sync Function files ({type(error).__name__}); retrying", flush=True)
        time.sleep(interval)


if __name__ == "__main__":
    main()
