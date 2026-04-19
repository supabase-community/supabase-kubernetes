#!/usr/bin/env python3
"""Sync image repositories/tags in values.yaml with Supabase docker-compose files."""

import re
import sys
import urllib.request
from typing import Dict, Optional, Tuple

DEFAULT_COMPOSE = "https://raw.githubusercontent.com/supabase/supabase/master/docker/docker-compose.yml"
DEFAULT_S3 = "https://raw.githubusercontent.com/supabase/supabase/master/docker/docker-compose.s3.yml"

SERVICE_MAP = {
    # values.yaml key -> docker-compose service name
    "analytics": "analytics",
    "auth": "auth",
    "db": "db",
    "functions": "functions",
    "imgproxy": "imgproxy",
    "kong": "kong",
    "meta": "meta",
    "minio": "minio",
    "realtime": "realtime",
    "rest": "rest",
    "storage": "storage",
    "studio": "studio",
    "vector": "vector",
}


class FetchError(RuntimeError):
    pass


def fetch_text(url: str) -> str:
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:
            data = resp.read()
    except Exception as exc:  # pragma: no cover - network dependent
        raise FetchError(f"failed to fetch {url}: {exc}")
    return data.decode("utf-8")


def parse_compose_images(text: str) -> Dict[str, str]:
    """Return service -> image string for a docker-compose YAML (text-based parse)."""
    images: Dict[str, str] = {}
    in_services = False
    current_service: Optional[str] = None

    for raw in text.splitlines():
        line = raw.rstrip("\n")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        if stripped == "services:":
            in_services = True
            current_service = None
            continue

        if not in_services:
            continue

        # service name (2-space indent)
        m_service = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
        if m_service:
            current_service = m_service.group(1)
            continue

        if current_service is None:
            continue

        m_image = re.match(r"^    image:\s*(.+?)\s*$", line)
        if m_image:
            image_value = m_image.group(1).strip().strip("'\"")
            images[current_service] = image_value
            continue

    return images


def split_image(image: str) -> Tuple[str, str]:
    """Split image into (repository, tag or digest). Defaults to 'latest' when tag is absent."""
    if "@" in image:
        repo, digest = image.split("@", 1)
        return repo, digest
    if ":" in image:
        repo, tag = image.rsplit(":", 1)
        if "/" in tag:
            # likely a registry port, not a tag
            return image, "latest"
        return repo, tag
    return image, "latest"


def build_updates(compose_images: Dict[str, str]) -> Dict[str, Tuple[str, str]]:
    updates: Dict[str, Tuple[str, str]] = {}
    for key, service in SERVICE_MAP.items():
        image = compose_images.get(service)
        if not image:
            continue
        repo, tag = split_image(image)
        updates[key] = (repo, tag)
    return updates


def update_values_yaml(values_path: str, updates: Dict[str, Tuple[str, str]]) -> None:
    """Update repository and tag lines inside image section."""
    with open(values_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    in_image = False
    current_key: Optional[str] = None

    for i, line in enumerate(lines):
        if re.match(r"^image:\s*$", line):
            in_image = True
            current_key = None
            continue

        if in_image:
            # end of image section when indent drops back to 0 and is not blank
            if re.match(r"^[^\s].*", line) and not line.startswith("image:"):
                in_image = False
                current_key = None
                continue

            m_key = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
            if m_key:
                current_key = m_key.group(1)
                continue

            if current_key and current_key in updates:
                repo, tag = updates[current_key]
                if re.match(r"^    repository:\s*", line):
                    lines[i] = f"    repository: {repo}\n"
                elif re.match(r"^    tag:\s*", line):
                    lines[i] = f'    tag: "{tag}"\n'

    with open(values_path, "w", encoding="utf-8") as f:
        f.writelines(lines)


def read_values_tags(values_path: str) -> Dict[str, str]:
    with open(values_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    tags: Dict[str, str] = {}
    in_image = False
    current_key: Optional[str] = None

    for line in lines:
        if re.match(r"^image:\s*$", line):
            in_image = True
            current_key = None
            continue

        if in_image:
            if re.match(r"^[^\s].*", line) and not line.startswith("image:"):
                in_image = False
                current_key = None
                continue

            m_key = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
            if m_key:
                current_key = m_key.group(1)
                continue

            if current_key:
                m_tag = re.match(r'^    tag:\s*"?([^"]+)"?\s*$', line)
                if m_tag:
                    tags[current_key] = m_tag.group(1)

    return tags


def main() -> int:
    values_path = "values.yaml"
    compose_url = DEFAULT_COMPOSE
    s3_url = DEFAULT_S3

    try:
        compose_text = fetch_text(compose_url)
        s3_text = fetch_text(s3_url)
    except FetchError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    compose_images = parse_compose_images(compose_text)
    compose_images.update(parse_compose_images(s3_text))

    updates = build_updates(compose_images)
    if not updates:
        print("No updates found. Check service mappings or compose files.")
        return 2

    current_tags = read_values_tags(values_path)
    changes = 0
    for key in sorted(updates.keys()):
        _, new_tag = updates[key]
        old_tag = current_tags.get(key, "(missing)")
        if old_tag != new_tag:
            print(f"{key}: {old_tag} -> {new_tag}")
            changes += 1

    if changes == 0:
        print("No updates found.")

    update_values_yaml(values_path, updates)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
