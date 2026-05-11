#!/usr/bin/env python3
"""Generate product images from visual descriptions via Gemini Nano Banana.

The script reads product visual profiles and generates images only for products
whose current image status needs replacement:

- likely_wrong
- no_image
- uncertain

It uses the Gemini API REST endpoint directly, so no third-party Python package
is required. Set GEMINI_API_KEY or GOOGLE_API_KEY before running.
"""

from __future__ import annotations

import argparse
import base64
import csv
import hashlib
import http.client
import json
import os
import random
import re
import sys
import time
import urllib.error
import urllib.request
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


DEFAULT_INPUT = Path("reports/product_visual_generation_profiles.csv")
DEFAULT_BRAND_PROFILES = Path("reports/brand_visual_profiles.json")
DEFAULT_OUTPUT_DIR = Path("reports/generated_product_images")
DEFAULT_MODEL = "gemini-2.5-flash-image"
DEFAULT_STATUSES = {"likely_wrong", "no_image", "uncertain"}
API_URL_TEMPLATE = "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent"
BATCH_API_URL_TEMPLATE = "https://generativelanguage.googleapis.com/v1beta/models/{model}:batchGenerateContent"
BATCH_GET_URL_TEMPLATE = "https://generativelanguage.googleapis.com/v1beta/{batch_name}"
FILE_UPLOAD_START_URL = "https://generativelanguage.googleapis.com/upload/v1beta/files"
FILE_DOWNLOAD_URL_TEMPLATE = "https://generativelanguage.googleapis.com/download/v1beta/{file_name}:download?alt=media"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate product images with Gemini Nano Banana from product visual descriptions.",
    )
    parser.add_argument("--input", type=Path, default=DEFAULT_INPUT, help="CSV table with product visual profiles.")
    parser.add_argument("--brand-profiles", type=Path, default=DEFAULT_BRAND_PROFILES, help="Brand profiles JSON.")
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR, help="Directory for generated images.")
    parser.add_argument("--model", default=DEFAULT_MODEL, help="Gemini image model. Nano Banana is gemini-2.5-flash-image.")
    parser.add_argument(
        "--action",
        choices=["submit-batch", "status", "download", "generate-one"],
        default="submit-batch",
        help="submit-batch creates a Gemini Batch job; status checks it; download saves result images.",
    )
    parser.add_argument("--batch-name", default="", help="Batch job name, for example batches/123456.")
    parser.add_argument("--responses-file", default="", help="Result file name from a succeeded batch, for example files/abc.")
    parser.add_argument("--display-name", default="", help="Optional Gemini batch display name.")
    parser.add_argument(
        "--statuses",
        default=",".join(sorted(DEFAULT_STATUSES)),
        help="Comma-separated current_image_status values to generate.",
    )
    parser.add_argument("--limit", type=int, default=0, help="Generate at most N images. 0 means no limit.")
    parser.add_argument("--offset", type=int, default=0, help="Skip the first N matching products.")
    parser.add_argument(
        "--one-per-brand",
        action="store_true",
        help="After filtering by status, keep only the first product for each brand.",
    )
    parser.add_argument(
        "--brand-profile-sample",
        action="store_true",
        help="Keep one product for every brand listed in brand_visual_profiles.json.",
    )
    parser.add_argument(
        "--brand-confidence",
        default="",
        help="For --brand-profile-sample, comma-separated confidence values to include. Empty means all.",
    )
    parser.add_argument("--sleep-sec", type=float, default=1.0, help="Pause between successful requests.")
    parser.add_argument("--retries", type=int, default=3, help="Retries per product after transient API errors.")
    parser.add_argument("--overwrite", action="store_true", help="Regenerate existing output files.")
    parser.add_argument("--dry-run", action="store_true", help="Print planned work without calling the API.")
    parser.add_argument("--wait", action="store_true", help="For status/download: poll until the batch reaches a terminal state.")
    parser.add_argument("--poll-sec", type=float, default=30.0, help="Polling interval when --wait is used.")
    parser.add_argument("--api-key-env", default="", help="Read API key from this env var instead of GEMINI_API_KEY/GOOGLE_API_KEY.")
    parser.add_argument(
        "--no-resume-download",
        action="store_true",
        help="Disable resume when downloading batch response files.",
    )
    return parser.parse_args()


def api_key_from_env(preferred_env: str) -> str:
    candidates = [preferred_env] if preferred_env else []
    candidates.extend(["GEMINI_API_KEY", "GOOGLE_API_KEY"])
    for name in candidates:
        if name and os.getenv(name):
            return os.environ[name]
    raise SystemExit("Gemini API key is missing. Set GEMINI_API_KEY or GOOGLE_API_KEY.")


def load_rows(path: Path, statuses: set[str] | None) -> list[dict[str, str]]:
    if not path.exists():
        raise SystemExit(f"Input table not found: {path}")
    with path.open(encoding="utf-8", newline="") as file:
        rows = list(csv.DictReader(file))
    missing = {"product_id", "name", "current_image_status"} - set(rows[0].keys() if rows else [])
    if missing:
        raise SystemExit(f"Input table is missing required columns: {', '.join(sorted(missing))}")
    if statuses is None:
        return rows
    return [row for row in rows if row.get("current_image_status") in statuses]


def one_product_per_brand(rows: list[dict[str, str]]) -> list[dict[str, str]]:
    selected: list[dict[str, str]] = []
    seen: set[str] = set()
    for row in rows:
        brand = (row.get("brand") or "Без бренда").strip().lower().replace("ё", "е")
        if brand in seen:
            continue
        seen.add(brand)
        selected.append(row)
    return selected


def brand_profile_sample_rows(
    rows: list[dict[str, str]],
    brand_profiles_path: Path,
    confidence_filter: set[str] | None,
) -> list[dict[str, str]]:
    if not brand_profiles_path.exists():
        raise SystemExit(f"Brand profiles file not found: {brand_profiles_path}")
    profiles = json.loads(brand_profiles_path.read_text(encoding="utf-8"))
    brands = [
        brand
        for brand, profile in profiles.items()
        if confidence_filter is None or profile.get("confidence") in confidence_filter
    ]
    by_brand: dict[str, list[dict[str, str]]] = defaultdict(list)
    for row in rows:
        by_brand[row.get("brand") or "Без бренда"].append(row)

    selected: list[dict[str, str]] = []
    missing: list[str] = []
    status_rank = {"no_image": 0, "uncertain": 1, "likely_wrong": 2, "likely_correct": 3}
    for brand in brands:
        candidates = by_brand.get(brand, [])
        if not candidates:
            missing.append(brand)
            continue
        candidates = sorted(
            candidates,
            key=lambda row: (
                status_rank.get(row.get("current_image_status", ""), 9),
                row.get("name", ""),
            ),
        )
        selected.append(candidates[0])
    if missing:
        print(f"warning: {len(missing)} brands from profiles have no products in input table", file=sys.stderr)
    return selected


def slugify(value: str, fallback: str = "product") -> str:
    value = value.lower().replace("ё", "е")
    value = re.sub(r"[^a-zа-я0-9]+", "-", value)
    value = value.strip("-")
    return value[:90] or fallback


def prompt_for(row: dict[str, str]) -> str:
    extended = row.get("расширенное описание") or row.get("extended_description_ru") or ""
    prompt = row.get("prompt_ru") or ""
    name = row.get("name", "")
    brand = row.get("brand", "")
    packaging = row.get("packaging_type", "")
    negative = row.get("negative_prompt_ru", "")

    visual_description = extended or prompt
    if not visual_description:
        visual_description = f"Фотореалистичное студийное фото товара {name}, бренд {brand}, упаковка {packaging}."

    return "\n".join(
        [
            "Сгенерируй одно фотореалистичное студийное изображение товара для каталога мобильного приложения.",
            "Нужен только один предмет упаковки на белом или очень светлом фоне.",
            "Ракурс фронтальный, мягкий ровный свет, товар полностью в кадре, без тени от других объектов.",
            "Сохрани тип упаковки, цвета бренда, крупное название товара, бренд и вес/объем, если они указаны.",
            "Описание товара:",
            visual_description,
            "Негативные ограничения:",
            negative or "без рук, без полки магазина, без интерьерного фона, без лишних предметов, без фантазийной упаковки",
        ]
    )


def extension_for_mime(mime_type: str) -> str:
    if mime_type == "image/jpeg":
        return ".jpg"
    if mime_type == "image/webp":
        return ".webp"
    return ".png"


def build_request(prompt: str) -> bytes:
    payload = {
        "contents": [
            {
                "role": "user",
                "parts": [{"text": prompt}],
            }
        ]
    }
    return json.dumps(payload, ensure_ascii=False).encode("utf-8")


def call_gemini(api_key: str, model: str, prompt: str, timeout_sec: float = 180.0) -> dict[str, Any]:
    url = API_URL_TEMPLATE.format(model=model)
    request = urllib.request.Request(
        url,
        data=build_request(prompt),
        headers={
            "Content-Type": "application/json",
            "x-goog-api-key": api_key,
        },
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=timeout_sec) as response:
        return json.loads(response.read().decode("utf-8"))


def http_json(
    url: str,
    api_key: str,
    payload: dict[str, Any] | None = None,
    method: str = "GET",
    timeout_sec: float = 180.0,
    retries: int = 1,
) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    last_error = ""
    for attempt in range(1, retries + 1):
        request = urllib.request.Request(
            url,
            data=data,
            headers={
                "Content-Type": "application/json",
                "x-goog-api-key": api_key,
            },
            method=method,
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout_sec) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as error:
            body = error.read().decode("utf-8", errors="replace")
            last_error = f"HTTP {error.code}: {body}"
            if error.code not in {408, 429, 500, 502, 503, 504} or attempt == retries:
                raise RuntimeError(last_error) from error
            retry_after = error.headers.get("Retry-After")
            if retry_after and retry_after.isdigit():
                sleep_sec = float(retry_after)
            else:
                sleep_sec = min(120.0, (2**attempt) + random.random())
            print(f"Transient API error while calling Gemini ({last_error[:180]}). Retrying in {sleep_sec:.1f}s...", file=sys.stderr)
            time.sleep(sleep_sec)
    raise RuntimeError(last_error or "Gemini request failed.")


def extract_image(response: dict[str, Any]) -> tuple[bytes, str, list[str]]:
    text_parts: list[str] = []
    for candidate in response.get("candidates", []):
        content = candidate.get("content", {})
        for part in content.get("parts", []):
            if "text" in part:
                text_parts.append(part["text"])
            inline = part.get("inlineData") or part.get("inline_data")
            if not inline:
                continue
            data = inline.get("data")
            if not data:
                continue
            mime_type = inline.get("mimeType") or inline.get("mime_type") or "image/png"
            return base64.b64decode(data), mime_type, text_parts
    raise RuntimeError("Gemini response did not contain image inlineData.")


def request_key(row: dict[str, str]) -> str:
    return f"{row['product_id']}__{slugify(row.get('name', 'product'))}"


def batch_request_for(row: dict[str, str]) -> dict[str, Any]:
    return {
        "key": request_key(row),
        "request": {
            "contents": [
                {
                    "role": "user",
                    "parts": [{"text": prompt_for(row)}],
                }
            ],
            "generation_config": {"responseModalities": ["TEXT", "IMAGE"]},
        },
    }


def write_batch_input(rows: list[dict[str, str]], output_dir: Path) -> tuple[Path, Path]:
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    jsonl_path = output_dir / f"gemini_batch_requests_{timestamp}.jsonl"
    map_path = output_dir / "batch_product_map.json"

    product_map: dict[str, dict[str, str]] = {}
    with jsonl_path.open("w", encoding="utf-8") as file:
        for row in rows:
            key = request_key(row)
            product_map[key] = {
                "product_id": row["product_id"],
                "name": row.get("name", ""),
                "brand": row.get("brand", ""),
                "status": row.get("current_image_status", ""),
                "slug": slugify(row.get("name", "product")),
            }
            file.write(json.dumps(batch_request_for(row), ensure_ascii=False) + "\n")

    map_path.write_text(json.dumps(product_map, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return jsonl_path, map_path


def upload_file(api_key: str, path: Path, display_name: str) -> dict[str, Any]:
    size = path.stat().st_size
    metadata = json.dumps({"file": {"display_name": display_name}}, ensure_ascii=False).encode("utf-8")
    start_request = urllib.request.Request(
        FILE_UPLOAD_START_URL,
        data=metadata,
        headers={
            "Content-Type": "application/json",
            "x-goog-api-key": api_key,
            "X-Goog-Upload-Protocol": "resumable",
            "X-Goog-Upload-Command": "start",
            "X-Goog-Upload-Header-Content-Length": str(size),
            "X-Goog-Upload-Header-Content-Type": "application/jsonl",
        },
        method="POST",
    )
    with urllib.request.urlopen(start_request, timeout=180.0) as response:
        upload_url = response.headers.get("x-goog-upload-url")
    if not upload_url:
        raise RuntimeError("File upload start response did not include x-goog-upload-url.")

    upload_request = urllib.request.Request(
        upload_url,
        data=path.read_bytes(),
        headers={
            "Content-Length": str(size),
            "X-Goog-Upload-Offset": "0",
            "X-Goog-Upload-Command": "upload, finalize",
        },
        method="POST",
    )
    with urllib.request.urlopen(upload_request, timeout=300.0) as response:
        return json.loads(response.read().decode("utf-8"))


def create_batch_job(api_key: str, model: str, file_name: str, display_name: str, retries: int) -> dict[str, Any]:
    payload = {
        "batch": {
            "display_name": display_name,
            "input_config": {"file_name": file_name},
        }
    }
    return http_json(
        BATCH_API_URL_TEMPLATE.format(model=model),
        api_key=api_key,
        payload=payload,
        method="POST",
        retries=retries,
    )


def get_batch(api_key: str, batch_name: str) -> dict[str, Any]:
    return http_json(BATCH_GET_URL_TEMPLATE.format(batch_name=batch_name), api_key=api_key)


def batch_state(batch: dict[str, Any]) -> str:
    return str(batch.get("state") or batch.get("metadata", {}).get("state") or "")


def is_batch_succeeded(state: str) -> bool:
    return state in {"JOB_STATE_SUCCEEDED", "BATCH_STATE_SUCCEEDED"}


def is_batch_failed(state: str) -> bool:
    return state in {"JOB_STATE_FAILED", "BATCH_STATE_FAILED"}


def is_batch_terminal(state: str) -> bool:
    return state in {
        "JOB_STATE_SUCCEEDED",
        "JOB_STATE_FAILED",
        "JOB_STATE_CANCELLED",
        "JOB_STATE_EXPIRED",
        "BATCH_STATE_SUCCEEDED",
        "BATCH_STATE_FAILED",
        "BATCH_STATE_CANCELLED",
        "BATCH_STATE_EXPIRED",
    }


def batch_responses_file(batch: dict[str, Any]) -> str:
    return str(
        batch.get("dest", {}).get("fileName")
        or batch.get("dest", {}).get("file_name")
        or batch.get("response", {}).get("responsesFile")
        or batch.get("response", {}).get("responses_file")
        or ""
    )


def parse_total_size_from_headers(response: http.client.HTTPResponse) -> int:
    content_range = response.headers.get("Content-Range", "")
    if "/" in content_range:
        tail = content_range.rsplit("/", 1)[-1].strip()
        if tail.isdigit():
            return int(tail)
    total_header = response.headers.get("Content-Length")
    if total_header and total_header.isdigit():
        return int(total_header)
    return 0


def download_file(api_key: str, file_name: str, retries: int, destination: Path, resume: bool = True) -> Path:
    url = FILE_DOWNLOAD_URL_TEMPLATE.format(file_name=file_name)
    last_error = ""
    destination.parent.mkdir(parents=True, exist_ok=True)
    for attempt in range(1, retries + 1):
        try:
            existing_bytes = destination.stat().st_size if resume and destination.exists() else 0
            headers = {"x-goog-api-key": api_key}
            if existing_bytes > 0:
                headers["Range"] = f"bytes={existing_bytes}-"
            request = urllib.request.Request(
                url,
                headers=headers,
                method="GET",
            )
            with urllib.request.urlopen(request, timeout=600.0) as response:
                status_code = getattr(response, "status", None) or response.getcode()
                total_size = parse_total_size_from_headers(response)
                if existing_bytes > 0 and status_code == 200:
                    # Server ignored Range; restart from scratch to avoid corrupt file append.
                    print("Server ignored Range resume request, restarting full download...", flush=True)
                    existing_bytes = 0
                file_mode = "ab" if existing_bytes > 0 and status_code == 206 else "wb"
                downloaded = existing_bytes
                print(
                    f"Downloading batch response {file_name} "
                    f"(attempt {attempt}/{retries}, expected_bytes={total_size or 'unknown'}, "
                    f"resume_from={existing_bytes})...",
                    flush=True,
                )
                with destination.open(file_mode) as output:
                    while True:
                        chunk = response.read(1024 * 1024)
                        if not chunk:
                            break
                        output.write(chunk)
                        downloaded += len(chunk)
                        if total_size > 0:
                            percent = downloaded * 100.0 / total_size
                            print(f"  downloaded {downloaded}/{total_size} bytes ({percent:.1f}%)", flush=True)
                        else:
                            print(f"  downloaded {downloaded} bytes", flush=True)
            print(f"Download completed: {downloaded} bytes", flush=True)
            return destination
        except http.client.IncompleteRead as error:
            partial_size = len(error.partial or b"")
            last_error = f"IncompleteRead(partial_bytes={partial_size})"
        except urllib.error.HTTPError as error:
            body = error.read().decode("utf-8", errors="replace")
            last_error = f"HTTP {error.code}: {body[:1200]}"
            if error.code == 416 and destination.exists():
                print("Range is not satisfiable; local file is likely complete. Reusing local download.", flush=True)
                return destination
            if error.code not in {408, 429, 500, 502, 503, 504}:
                raise RuntimeError(last_error) from error
        except Exception as error:  # noqa: BLE001 - keep retry behavior for transient network issues.
            last_error = str(error)

        if attempt < retries:
            sleep_sec = min(120.0, (2**attempt) + random.random())
            print(
                f"Transient download error for {file_name} ({last_error}). Retrying in {sleep_sec:.1f}s...",
                file=sys.stderr,
            )
            time.sleep(sleep_sec)

    raise RuntimeError(f"Failed to download {file_name}: {last_error}")


def load_product_map(output_dir: Path) -> dict[str, dict[str, str]]:
    path = output_dir / "batch_product_map.json"
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def process_batch_results(results_bytes: bytes, output_dir: Path, model: str) -> tuple[int, int]:
    product_map = load_product_map(output_dir)
    result_path = output_dir / "batch_results.jsonl"
    result_path.write_bytes(results_bytes)

    saved = 0
    failed = 0
    manifest_path = output_dir / "manifest.jsonl"
    lines = results_bytes.decode("utf-8", errors="replace").splitlines()
    total = len(lines)
    print(f"Processing batch results: {total} lines", flush=True)
    for index, line in enumerate(lines, start=1):
        if not line.strip():
            continue
        if index % 25 == 0 or index == total:
            print(f"  processed {index}/{total} lines", flush=True)
        try:
            parsed = json.loads(line)
        except json.JSONDecodeError as error:
            failed += 1
            append_manifest(
                manifest_path,
                {
                    "ok": False,
                    "product_id": "",
                    "name": "",
                    "status": "",
                    "model": model,
                    "batch_key": "",
                    "error": f"Malformed JSON line {index}: {error}",
                    "generated_at": datetime.now(timezone.utc).isoformat(),
                },
            )
            continue
        key = parsed.get("key") or parsed.get("metadata", {}).get("key") or ""
        mapped = product_map.get(key, {})
        product_id = mapped.get("product_id") or key.split("__", 1)[0]
        slug = mapped.get("slug") or slugify(mapped.get("name", product_id))
        name = mapped.get("name", "")
        status = mapped.get("status", "")

        if parsed.get("error"):
            failed += 1
            append_manifest(
                manifest_path,
                {
                    "ok": False,
                    "product_id": product_id,
                    "name": name,
                    "status": status,
                    "model": model,
                    "batch_key": key,
                    "error": parsed["error"],
                    "generated_at": datetime.now(timezone.utc).isoformat(),
                },
            )
            continue

        response = parsed.get("response") or parsed.get("inlineResponse") or parsed.get("inline_response")
        if not response:
            failed += 1
            append_manifest(
                manifest_path,
                {
                    "ok": False,
                    "product_id": product_id,
                    "name": name,
                    "status": status,
                    "model": model,
                    "batch_key": key,
                    "error": "Result line did not contain response.",
                    "generated_at": datetime.now(timezone.utc).isoformat(),
                },
            )
            continue

        try:
            image_bytes, mime_type, text_parts = extract_image(response)
            output_path = output_dir / f"{product_id}_{slug}{extension_for_mime(mime_type)}"
            output_path.write_bytes(image_bytes)
            saved += 1
            append_manifest(
                manifest_path,
                {
                    "ok": True,
                    "product_id": product_id,
                    "name": name,
                    "status": status,
                    "model": model,
                    "batch_key": key,
                    "output_path": str(output_path),
                    "mime_type": mime_type,
                    "usage_metadata": response.get("usageMetadata") or response.get("usage_metadata") or {},
                    "response_text": "\n".join(text_parts).strip(),
                    "generated_at": datetime.now(timezone.utc).isoformat(),
                },
            )
        except Exception as error:  # noqa: BLE001 - continue saving other batch results.
            failed += 1
            append_manifest(
                manifest_path,
                {
                    "ok": False,
                    "product_id": product_id,
                    "name": name,
                    "status": status,
                    "model": model,
                    "batch_key": key,
                    "error": str(error),
                    "generated_at": datetime.now(timezone.utc).isoformat(),
                },
            )
    return saved, failed


def existing_manifest_ids(path: Path) -> set[str]:
    if not path.exists():
        return set()
    ids: set[str] = set()
    with path.open(encoding="utf-8") as file:
        for line in file:
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            if row.get("ok") and row.get("product_id"):
                ids.add(row["product_id"])
    return ids


def append_manifest(path: Path, row: dict[str, Any]) -> None:
    with path.open("a", encoding="utf-8") as file:
        file.write(json.dumps(row, ensure_ascii=False) + "\n")


def generate_one(api_key: str, model: str, row: dict[str, str], output_dir: Path, retries: int) -> dict[str, Any]:
    prompt = prompt_for(row)
    prompt_hash = hashlib.sha256(prompt.encode("utf-8")).hexdigest()[:16]
    base_name = f"{row['product_id']}_{slugify(row.get('name', 'product'))}"

    last_error = ""
    for attempt in range(1, retries + 1):
        try:
            response = call_gemini(api_key=api_key, model=model, prompt=prompt)
            image_bytes, mime_type, text_parts = extract_image(response)
            output_path = output_dir / f"{base_name}{extension_for_mime(mime_type)}"
            output_path.write_bytes(image_bytes)
            return {
                "ok": True,
                "product_id": row["product_id"],
                "name": row.get("name", ""),
                "status": row.get("current_image_status", ""),
                "model": model,
                "output_path": str(output_path),
                "mime_type": mime_type,
                "prompt_hash": prompt_hash,
                "usage_metadata": response.get("usageMetadata") or response.get("usage_metadata") or {},
                "response_text": "\n".join(text_parts).strip(),
                "generated_at": datetime.now(timezone.utc).isoformat(),
            }
        except urllib.error.HTTPError as error:
            body = error.read().decode("utf-8", errors="replace")
            last_error = f"HTTP {error.code}: {body[:1200]}"
            if error.code not in {408, 429, 500, 502, 503, 504}:
                break
        except Exception as error:  # noqa: BLE001 - manifest should capture per-item failures.
            last_error = str(error)
        if attempt < retries:
            time.sleep(min(30.0, (2**attempt) + random.random()))

    return {
        "ok": False,
        "product_id": row["product_id"],
        "name": row.get("name", ""),
        "status": row.get("current_image_status", ""),
        "model": model,
        "error": last_error,
        "generated_at": datetime.now(timezone.utc).isoformat(),
    }


def main() -> int:
    args = parse_args()
    api_key = api_key_from_env(args.api_key_env) if not args.dry_run else ""

    if args.action == "status":
        if not args.batch_name:
            raise SystemExit("--batch-name is required for --action status")
        while True:
            batch = get_batch(api_key, args.batch_name)
            state = batch_state(batch)
            print(json.dumps(batch, ensure_ascii=False, indent=2))
            if not args.wait or is_batch_terminal(state):
                return 0 if not is_batch_failed(state) else 1
            time.sleep(args.poll_sec)

    if args.action == "download":
        if not args.batch_name and not args.responses_file:
            raise SystemExit("--batch-name or --responses-file is required for --action download")
        responses_file = args.responses_file
        if not responses_file:
            while True:
                batch = get_batch(api_key, args.batch_name)
                state = batch_state(batch)
                print(f"batch={args.batch_name} state={state}")
                if is_batch_succeeded(state):
                    responses_file = batch_responses_file(batch)
                    break
                if not args.wait or is_batch_terminal(state):
                    raise SystemExit(f"Batch is not ready for download. state={state}")
                time.sleep(args.poll_sec)
        if not responses_file:
            raise SystemExit("Succeeded batch did not expose a responses file.")
        args.output_dir.mkdir(parents=True, exist_ok=True)
        result_path = args.output_dir / "batch_results.jsonl"
        download_file(
            api_key=api_key,
            file_name=responses_file,
            retries=args.retries,
            destination=result_path,
            resume=not args.no_resume_download,
        )
        results = result_path.read_bytes()
        saved, failed = process_batch_results(results, args.output_dir, args.model)
        print(f"downloaded={responses_file}")
        print(f"saved_images={saved}")
        print(f"failed_results={failed}")
        print(f"manifest={args.output_dir / 'manifest.jsonl'}")
        return 1 if failed else 0

    statuses = None if args.brand_profile_sample else {status.strip() for status in args.statuses.split(",") if status.strip()}
    rows = load_rows(args.input, statuses)
    if args.brand_profile_sample:
        confidence_filter = (
            {value.strip() for value in args.brand_confidence.split(",") if value.strip()}
            if args.brand_confidence
            else None
        )
        rows = brand_profile_sample_rows(rows, args.brand_profiles, confidence_filter)
    elif args.one_per_brand:
        rows = one_product_per_brand(rows)
    rows = rows[args.offset :]
    if args.limit > 0:
        rows = rows[: args.limit]

    args.output_dir.mkdir(parents=True, exist_ok=True)
    manifest_path = args.output_dir / "manifest.jsonl"
    done_ids = set() if args.overwrite else existing_manifest_ids(manifest_path)
    rows = [row for row in rows if args.overwrite or row["product_id"] not in done_ids]

    print(f"input={args.input}")
    print(f"output_dir={args.output_dir}")
    print(f"model={args.model}")
    print(f"statuses={'all' if statuses is None else ','.join(sorted(statuses))}")
    print(f"to_generate={len(rows)}")

    if args.dry_run:
        for row in rows[:20]:
            print(f"DRY {row['current_image_status']} {row['product_id']} {row.get('name', '')}")
        if len(rows) > 20:
            print(f"... and {len(rows) - 20} more")
        return 0

    if args.action == "submit-batch":
        batch_input_path, product_map_path = write_batch_input(rows, args.output_dir)
        display_name = args.display_name or f"foodsea-product-images-{datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S')}"
        uploaded = upload_file(api_key, batch_input_path, display_name)
        file_name = uploaded.get("file", {}).get("name")
        if not file_name:
            raise SystemExit(f"Upload response did not include file.name: {uploaded}")
        batch = create_batch_job(api_key, args.model, file_name, display_name, args.retries)
        batch_info_path = args.output_dir / "batch_job.json"
        batch_info_path.write_text(json.dumps(batch, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"batch_input={batch_input_path}")
        print(f"product_map={product_map_path}")
        print(f"uploaded_file={file_name}")
        print(f"batch_name={batch.get('name', '')}")
        print(f"batch_job={batch_info_path}")
        return 0

    if args.action != "generate-one":
        raise SystemExit(f"Unsupported action: {args.action}")

    failures = 0
    for index, row in enumerate(rows, start=1):
        print(f"[{index}/{len(rows)}] {row.get('current_image_status')} {row.get('name')}", flush=True)
        result = generate_one(api_key, args.model, row, args.output_dir, args.retries)
        append_manifest(manifest_path, result)
        if not result["ok"]:
            failures += 1
            print(f"  failed: {result.get('error', '')[:300]}", file=sys.stderr)
        elif args.sleep_sec > 0:
            time.sleep(args.sleep_sec)

    print(f"done failures={failures} manifest={manifest_path}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
