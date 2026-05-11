#!/usr/bin/env python3
"""Build image candidate reports for products without barcodes.

The script does not write to DB. It scans seed JSON files, samples products
without barcode, searches the web, extracts candidate images/pages, and writes
confidence-scored report to JSON.
"""

from __future__ import annotations

import argparse
import base64
import csv
import dataclasses
import datetime as dt
import html
import json
import random
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


DEFAULT_PRODUCTS_DIR = Path("/Users/mihailbasin/PycharmProjects/GenerateProductData/products")
DEFAULT_BARCODES_FILE = Path("/Users/mihailbasin/PycharmProjects/GenerateProductData/barcode_progress.json")
DEFAULT_OUTPUT = Path("reports/image-candidate-report.json")

USER_AGENT = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)

STOP_WORDS = {
    "и",
    "в",
    "на",
    "с",
    "для",
    "из",
    "по",
    "к",
    "товар",
    "продукт",
    "шт",
    "уп",
    "упаковка",
    "г",
    "гр",
    "л",
    "мл",
}

TRUSTED_DOMAINS = {
    "winemore.ru",
    "chizhikmagazin.ru",
    "shop.samberi.com",
    "lenta.com",
    "lenta.ru",
    "perekrestok.ru",
    "magnit.ru",
    "5ka.ru",
    "svetoforonline.ru",
    "svetoforu.ru",
    "miratorg.ru",
    "vprok.ru",
}

BING_NAV_LABELS = {
    "all",
    "search",
    "images",
    "videos",
    "maps",
    "news",
    "shopping",
    "more",
    "tools",
    "past 24 hours",
    "past week",
    "past month",
}

URL_PATH_HINTS = {
    "product",
    "products",
    "catalog",
    "catalogue",
    "item",
    "goods",
    "sku",
    "card",
    "offer",
}


@dataclasses.dataclass(frozen=True)
class ProductRecord:
    product_id: str
    name: str
    brand: str
    category: str
    subcategory: str
    source_file: str


@dataclasses.dataclass(frozen=True)
class SearchResult:
    title: str
    page_url: str
    image_url_hint: str | None = None


def fetch_text(url: str, timeout: float = 12.0) -> str:
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT, "Accept-Language": "ru,en;q=0.8"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read()
    return body.decode("utf-8", errors="replace")


def normalize_text(value: str) -> str:
    lowered = value.lower().replace("ё", "е")
    lowered = re.sub(r"[^a-zа-я0-9\.\,\s%]", " ", lowered)
    lowered = re.sub(r"\s+", " ", lowered).strip()
    return lowered


def extract_volume(value: str) -> str | None:
    norm = normalize_text(value).replace(",", ".")
    match = re.search(r"(\d+(?:\.\d+)?)\s*(мл|л|г|гр|кг)", norm)
    if not match:
        return None
    number, unit = match.groups()
    if unit == "гр":
        unit = "г"
    return f"{number}{unit}"


def tokenize(value: str) -> list[str]:
    norm = normalize_text(value)
    parts = [p for p in norm.split(" ") if p and p not in STOP_WORDS and len(p) > 1]
    return parts


def query_search_results(query: str, max_results: int, pause_sec: float, search_timeout_sec: float) -> list[SearchResult]:
    results = query_bing_results(query=query, max_results=max_results, timeout_sec=search_timeout_sec)
    if not results:
        results = query_ddg_results(query=query, max_results=max_results, timeout_sec=search_timeout_sec)
    if pause_sec > 0:
        time.sleep(pause_sec)
    return results


def query_bing_results(query: str, max_results: int, timeout_sec: float) -> list[SearchResult]:
    encoded = urllib.parse.quote_plus(query)
    # r.jina.ai returns a markdown snapshot that bypasses client-side rendering.
    url = f"https://r.jina.ai/http://www.bing.com/search?q={encoded}&setlang=ru"
    body = fetch_text(url, timeout=timeout_sec)

    results: list[SearchResult] = []
    pattern = re.compile(r"\[([^\]]+)\]\((https?://[^)]+)\)")
    seen: set[str] = set()
    for title_raw, href_raw in pattern.findall(body):
        href = extract_bing_target(html.unescape(href_raw))
        title = html.unescape(title_raw).strip()
        if title.lower() in BING_NAV_LABELS:
            continue
        if len(title) < 4:
            continue
        if not href.startswith("http://") and not href.startswith("https://"):
            continue
        if "bing.com/" in href:
            continue
        if href in seen:
            continue
        seen.add(href)
        results.append(SearchResult(title=title, page_url=href))
        if len(results) >= max_results:
            break
    return results


def extract_bing_target(url: str) -> str:
    parsed = urllib.parse.urlparse(url)
    host = parsed.netloc.lower()
    if "bing.com" not in host:
        return url
    qs = urllib.parse.parse_qs(parsed.query)
    encoded = qs.get("u", [])
    if not encoded:
        return url
    raw = encoded[0]
    if not raw.startswith("a1"):
        return url
    payload = raw[2:]
    try:
        pad = "=" * ((4 - (len(payload) % 4)) % 4)
        decoded = base64.b64decode(payload + pad).decode("utf-8", errors="replace")
        if decoded.startswith("http://") or decoded.startswith("https://"):
            return decoded
    except Exception:
        return url
    return url


def query_ddg_results(query: str, max_results: int, timeout_sec: float) -> list[SearchResult]:
    encoded = urllib.parse.quote_plus(query)
    url = f"https://duckduckgo.com/html/?q={encoded}&kl=ru-ru"
    body = fetch_text(url, timeout=timeout_sec)
    # DuckDuckGo HTML search wraps link into ...uddg=<url-encoded-target>.
    pattern = re.compile(r'<a[^>]+href="(?P<href>[^"]+)"[^>]*>(?P<title>.*?)</a>', re.IGNORECASE | re.DOTALL)
    results: list[SearchResult] = []
    for match in pattern.finditer(body):
        href = html.unescape(match.group("href"))
        title = re.sub(r"<.*?>", "", match.group("title"))
        title = html.unescape(title).strip()
        target = extract_uddg_target(href)
        if not target:
            continue
        # Drop internal DuckDuckGo redirect/help pages.
        if "duckduckgo.com/" in target and "uddg=" not in target:
            continue
        results.append(SearchResult(title=title, page_url=target))
        if len(results) >= max_results:
            break
    return results


def query_magnit_domain_results(query: str, max_results: int, timeout_sec: float) -> list[SearchResult]:
    encoded = urllib.parse.quote_plus(query)
    url = f"https://magnit.ru/search/?q={encoded}"
    body = fetch_text(url, timeout=timeout_sec)

    # Magnit server-rendered catalog cards contain title + product href + image.
    pattern = re.compile(
        r'<a\s+title="([^"]+)"[^>]*href="(/product/[^"]+)"[^>]*>[\s\S]*?<img\s+src="([^"]+)"',
        re.IGNORECASE,
    )
    rows: list[SearchResult] = []
    seen: set[str] = set()
    query_tokens = set(tokenize(query))
    scored: list[tuple[int, SearchResult]] = []

    for title_raw, href_raw, img_raw in pattern.findall(body):
        title = html.unescape(title_raw).strip()
        href = urllib.parse.urljoin("https://magnit.ru", html.unescape(href_raw))
        img = html.unescape(img_raw)
        if href in seen:
            continue
        seen.add(href)
        title_tokens = set(tokenize(title))
        overlap = len(query_tokens & title_tokens)
        if overlap < 2:
            continue
        scored.append((overlap, SearchResult(title=title, page_url=href, image_url_hint=img)))

    # Prefer greater token overlap with query.
    for _, item in sorted(scored, key=lambda x: x[0], reverse=True)[: max_results]:
        rows.append(item)
    return rows


def extract_uddg_target(link: str) -> str | None:
    if "uddg=" not in link:
        if link.startswith("http://") or link.startswith("https://"):
            return link
        return None
    parsed = urllib.parse.urlparse(link)
    qs = urllib.parse.parse_qs(parsed.query)
    uddg_values = qs.get("uddg")
    if not uddg_values:
        return None
    return urllib.parse.unquote(uddg_values[0])


def score_candidate(
    *,
    product: ProductRecord,
    title: str,
    page_url: str,
    page_text: str,
    image_url: str | None,
) -> dict[str, Any]:
    reasons: list[str] = []
    penalties: list[str] = []
    score = 0

    combined = " ".join([title, page_text[:3000]])
    combined_norm = normalize_text(combined)
    title_norm = normalize_text(title)
    brand_norm = normalize_text(product.brand)
    prod_tokens = tokenize(product.name)
    volume = extract_volume(product.name)

    if brand_norm and brand_norm in combined_norm:
        score += 25
        reasons.append("brand_exact_match")

    if prod_tokens:
        matched = sum(1 for token in prod_tokens if token in combined_norm)
        ratio = matched / max(len(prod_tokens), 1)
        if ratio >= 0.75:
            score += 20
            reasons.append("name_tokens_match_high")
        elif ratio >= 0.45:
            score += 10
            reasons.append("name_tokens_match_partial")
        elif ratio < 0.2:
            penalties.append("name_tokens_low_match")
            score -= 10

    if volume:
        v_num, v_unit = re.match(r"([0-9.]+)(.+)", volume).groups()  # type: ignore[union-attr]
        if f"{v_num}{v_unit}" in combined_norm or f"{v_num} {v_unit}" in combined_norm:
            score += 15
            reasons.append("volume_match")
        else:
            score -= 8
            penalties.append("volume_mismatch")

    category_token = normalize_text(product.category)
    if category_token and category_token in combined_norm:
        score += 10
        reasons.append("category_match")

    if is_product_page(title_norm, combined_norm):
        score += 10
        reasons.append("product_page_pattern")

    host = urllib.parse.urlparse(page_url).netloc.lower().replace("www.", "")
    if is_allowed_domain(host, TRUSTED_DOMAINS):
        score += 5
        reasons.append("trusted_domain")
    else:
        score -= 15
        penalties.append("untrusted_domain")

    if looks_like_product_card(page_url, title):
        score += 10
        reasons.append("product_card_url_hint")
    else:
        score -= 12
        penalties.append("not_product_card_url")

    if image_url:
        score += 5
        reasons.append("image_extracted")
    else:
        score -= 12
        penalties.append("no_image_extracted")

    if product.brand and brand_norm and brand_norm not in title_norm and brand_norm not in combined_norm:
        score -= 20
        penalties.append("brand_missing")

    score = max(0, min(score, 100))
    return {"score": score, "reasons": reasons, "penalties": penalties}


def is_product_page(title_norm: str, combined_norm: str) -> bool:
    patterns = ["купить", "цена", "в наличии", "объем", "состав", "характеристик", "товар"]
    return any(p in title_norm or p in combined_norm for p in patterns)


def looks_like_product_card(page_url: str, title: str) -> bool:
    host = host_from_url(page_url)
    path = urllib.parse.urlparse(page_url).path.lower()
    title_norm = normalize_text(title)
    if any(f"/{hint}" in path for hint in URL_PATH_HINTS):
        return True
    # Domain-specific catalog markers.
    if host.endswith("magnit.ru") and "/product/" in path:
        return True
    if host.endswith("perekrestok.ru") and ("/cat/" in path or "/product/" in path):
        return True
    if host.endswith("5ka.ru") and ("/catalog/" in path or "/product/" in path):
        return True
    if host.endswith("lenta.com") or host.endswith("lenta.ru"):
        if "/product/" in path or "/catalog/" in path:
            return True
    if host.endswith("chizhikmagazin.ru") and "/catalogue/" in path:
        return True
    if host.endswith("miratorg.ru") and ("/catalog/" in path or "/product/" in path):
        return True
    return any(k in title_norm for k in ["купить", "цена", "товар"])


def extract_image_url(page_html: str, page_url: str) -> str | None:
    # Prefer OpenGraph image.
    og_match = re.search(
        r'<meta[^>]+property=["\']og:image["\'][^>]+content=["\']([^"\']+)["\']',
        page_html,
        re.IGNORECASE,
    )
    if og_match:
        return urllib.parse.urljoin(page_url, html.unescape(og_match.group(1)))

    # Fallback to first meaningful image.
    img_match = re.search(r'<img[^>]+src=["\']([^"\']+)["\']', page_html, re.IGNORECASE)
    if img_match:
        return urllib.parse.urljoin(page_url, html.unescape(img_match.group(1)))

    return None


def collect_products_without_barcode(products_dir: Path, barcode_values: dict[str, Any]) -> list[ProductRecord]:
    products: dict[str, ProductRecord] = {}
    for path in products_dir.rglob("*.json"):
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            continue
        if not isinstance(raw, list):
            continue

        for item in raw:
            if not isinstance(item, dict):
                continue
            pid = str(item.get("id", "")).strip()
            if not pid or pid in products:
                continue
            if barcode_values.get(pid):
                continue
            products[pid] = ProductRecord(
                product_id=pid,
                name=str(item.get("name", "")).strip(),
                brand=str(item.get("brand", "")).strip(),
                category=str(item.get("category", "")).strip(),
                subcategory=str(item.get("subcategory", "")).strip(),
                source_file=str(path),
            )
    return list(products.values())


def build_query(product: ProductRecord) -> str:
    volume = extract_volume(product.name) or ""
    parts = [product.brand, product.name, product.category, volume, "фото товара"]
    return " ".join(p for p in parts if p).strip()


def build_query_variants(product: ProductRecord) -> list[str]:
    volume = extract_volume(product.name) or ""
    variants = [
        " ".join(p for p in [product.brand, product.name, volume, "фото товара"] if p).strip(),
        " ".join(p for p in [product.brand, product.name] if p).strip(),
        " ".join(p for p in [product.name, "фото товара"] if p).strip(),
    ]
    # Preserve order and uniqueness.
    seen: set[str] = set()
    out: list[str] = []
    for variant in variants:
        cleaned = variant.strip()
        if not cleaned or cleaned in seen:
            continue
        seen.add(cleaned)
        out.append(cleaned)
    return out


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build image candidate report for products without barcode")
    parser.add_argument("--products-dir", type=Path, default=DEFAULT_PRODUCTS_DIR)
    parser.add_argument("--barcodes-file", type=Path, default=DEFAULT_BARCODES_FILE)
    parser.add_argument("--out", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--csv-out", type=Path, default=None)
    parser.add_argument("--limit", type=int, default=20)
    parser.add_argument("--max-search-results", type=int, default=5, help="Max candidates to fetch per product")
    parser.add_argument("--search-pool-size", type=int, default=20, help="Raw search results to collect before filtering")
    parser.add_argument("--max-queries-per-domain", type=int, default=2)
    parser.add_argument("--max-results-per-domain", type=int, default=1)
    parser.add_argument(
        "--allowed-domains",
        type=str,
        default=",".join(sorted(TRUSTED_DOMAINS)),
        help="Comma-separated allowed domains (subdomains are allowed)",
    )
    parser.add_argument("--auto-accept-threshold", type=int, default=85)
    parser.add_argument("--pending-threshold", type=int, default=55)
    parser.add_argument("--shuffle-seed", type=int, default=42)
    parser.add_argument("--pause-sec", type=float, default=0.35)
    parser.add_argument("--search-timeout-sec", type=float, default=12.0)
    parser.add_argument("--page-timeout-sec", type=float, default=8.0)
    parser.add_argument("--strict-product-url-hint", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    if not args.products_dir.exists():
        print(f"products directory does not exist: {args.products_dir}", file=sys.stderr)
        return 2
    if not args.barcodes_file.exists():
        print(f"barcodes file does not exist: {args.barcodes_file}", file=sys.stderr)
        return 2

    barcodes_raw = json.loads(args.barcodes_file.read_text(encoding="utf-8"))
    if not isinstance(barcodes_raw, dict):
        print("barcodes file must be a JSON object", file=sys.stderr)
        return 2

    products = collect_products_without_barcode(args.products_dir, barcodes_raw)
    if not products:
        print("no products without barcode found")
        return 0

    random.seed(args.shuffle_seed)
    random.shuffle(products)
    sample = products[: max(0, args.limit)]
    allowed_domains = parse_allowed_domains(args.allowed_domains)
    csv_out = args.csv_out or args.out.with_suffix(".csv")

    report_items: list[dict[str, Any]] = []
    for index, product in enumerate(sample, start=1):
        query = build_query(product)
        query_variants = build_query_variants(product)
        item: dict[str, Any] = dataclasses.asdict(product) | {
            "query": query,
            "query_variants": query_variants,
            "candidates": [],
            "errors": [],
        }
        search_results: list[SearchResult] = []
        seen_urls: set[str] = set()
        for domain in sorted(allowed_domains):
            domain_hits = 0
            for q in query_variants[: max(1, args.max_queries_per_domain)]:
                site_query = f"site:{domain} {q}"
                try:
                    if domain == "magnit.ru":
                        batch = query_magnit_domain_results(
                            query=q,
                            max_results=max(1, args.search_pool_size),
                            timeout_sec=max(1.0, args.search_timeout_sec),
                        )
                        if not batch:
                            batch = query_search_results(
                                query=site_query,
                                max_results=max(1, args.search_pool_size),
                                pause_sec=max(0.0, args.pause_sec),
                                search_timeout_sec=max(1.0, args.search_timeout_sec),
                            )
                    else:
                        batch = query_search_results(
                            query=site_query,
                            max_results=max(1, args.search_pool_size),
                            pause_sec=max(0.0, args.pause_sec),
                            search_timeout_sec=max(1.0, args.search_timeout_sec),
                        )
                except (urllib.error.URLError, TimeoutError) as err:
                    item["errors"].append(f"search_failed[{domain}|{q}]: {err}")
                    continue
                except Exception as err:  # pragma: no cover
                    item["errors"].append(f"search_failed_unexpected[{domain}|{q}]: {err}")
                    continue

                for result in batch:
                    if result.page_url in seen_urls:
                        continue
                    result_host = host_from_url(result.page_url)
                    if not same_domain(result_host, domain):
                        continue
                    if args.strict_product_url_hint and not looks_like_product_card(result.page_url, result.title):
                        continue
                    seen_urls.add(result.page_url)
                    search_results.append(result)
                    domain_hits += 1
                    if domain_hits >= max(1, args.max_results_per_domain):
                        break
                    if len(search_results) >= max(1, args.max_search_results):
                        break

                if domain_hits >= max(1, args.max_results_per_domain):
                    break
                if len(search_results) >= max(1, args.max_search_results):
                    break
            if len(search_results) >= max(1, args.max_search_results):
                break

        search_results = search_results[: max(1, args.max_search_results)]

        for search_result in search_results:
            candidate: dict[str, Any] = {"title": search_result.title, "page_url": search_result.page_url}
            domain = host_from_url(search_result.page_url)
            candidate["domain"] = domain
            candidate["allowed_domain"] = is_allowed_domain(domain, allowed_domains)
            try:
                if search_result.image_url_hint:
                    page_html = search_result.title
                    image_url = search_result.image_url_hint
                else:
                    page_html = fetch_text(search_result.page_url, timeout=max(1.0, args.page_timeout_sec))
                    image_url = extract_image_url(page_html, search_result.page_url)
                score = score_candidate(
                    product=product,
                    title=search_result.title,
                    page_url=search_result.page_url,
                    page_text=page_html,
                    image_url=image_url,
                )
                candidate["image_url"] = image_url
                candidate["confidence"] = score["score"]
                candidate["reasons"] = score["reasons"]
                candidate["penalties"] = score["penalties"]
            except Exception as err:  # pragma: no cover
                candidate["error"] = str(err)
                candidate["confidence"] = 0
                candidate["reasons"] = []
                candidate["penalties"] = ["page_fetch_failed"]
            item["candidates"].append(candidate)

        item["candidates"].sort(key=lambda x: int(x.get("confidence", 0)), reverse=True)
        top = item["candidates"][0] if item["candidates"] else None
        if top is not None:
            status = classify_candidate(
                top,
                auto_accept_threshold=max(0, min(100, args.auto_accept_threshold)),
                pending_threshold=max(0, min(100, args.pending_threshold)),
            )
            item["top_candidate_status"] = status
            item["top_candidate_confidence"] = int(top.get("confidence", 0))
            item["top_candidate_url"] = top.get("page_url")
            item["top_candidate_domain"] = top.get("domain")
        else:
            item["top_candidate_status"] = "no_candidates"
            item["top_candidate_confidence"] = 0
            item["top_candidate_url"] = None
            item["top_candidate_domain"] = None
        report_items.append(item)
        print(
            f"[{index}/{len(sample)}] {product.name}: {len(item['candidates'])} candidates "
            f"| status={item['top_candidate_status']} conf={item['top_candidate_confidence']}"
        )

    args.out.parent.mkdir(parents=True, exist_ok=True)
    summary = {
        "generated_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "products_dir": str(args.products_dir),
        "barcodes_file": str(args.barcodes_file),
        "sample_size": len(sample),
        "source_pool_without_barcode": len(products),
        "report_items": len(report_items),
        "allowed_domains": sorted(allowed_domains),
        "auto_accept_threshold": max(0, min(100, args.auto_accept_threshold)),
        "pending_threshold": max(0, min(100, args.pending_threshold)),
        "status_counts": count_statuses(report_items),
    }
    payload = {"summary": summary, "items": report_items}
    args.out.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    write_top_candidates_csv(report_items, csv_out)

    print(f"report written to: {args.out}")
    print(f"csv written to: {csv_out}")
    return 0


def parse_allowed_domains(raw: str) -> set[str]:
    out: set[str] = set()
    for part in raw.split(","):
        domain = part.strip().lower()
        if not domain:
            continue
        out.add(domain.removeprefix("www."))
    return out


def host_from_url(url: str) -> str:
    try:
        host = urllib.parse.urlparse(url).netloc.lower()
    except Exception:
        return ""
    return host.removeprefix("www.")


def is_allowed_domain(host: str, allowed_domains: set[str]) -> bool:
    if not host:
        return False
    for domain in allowed_domains:
        if host == domain or host.endswith("." + domain):
            return True
    return False


def same_domain(host: str, domain: str) -> bool:
    return host == domain or host.endswith("." + domain)


def classify_candidate(candidate: dict[str, Any], auto_accept_threshold: int, pending_threshold: int) -> str:
    confidence = int(candidate.get("confidence", 0))
    has_error = bool(candidate.get("error"))
    has_image = bool(candidate.get("image_url"))
    allowed_domain = bool(candidate.get("allowed_domain"))

    if has_error or not has_image or not allowed_domain:
        return "rejected"
    if confidence >= auto_accept_threshold:
        return "auto_accepted"
    if confidence >= pending_threshold:
        return "pending_review"
    return "rejected"


def count_statuses(items: list[dict[str, Any]]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for item in items:
        key = str(item.get("top_candidate_status", "unknown"))
        counts[key] = counts.get(key, 0) + 1
    return counts


def write_top_candidates_csv(items: list[dict[str, Any]], out_path: Path) -> None:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("w", encoding="utf-8", newline="") as f:
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "product_id",
                "name",
                "brand",
                "category",
                "subcategory",
                "query",
                "status",
                "confidence",
                "domain",
                "page_url",
                "image_url",
                "reasons",
                "penalties",
                "error",
            ],
        )
        writer.writeheader()
        for item in items:
            top = item["candidates"][0] if item.get("candidates") else {}
            writer.writerow(
                {
                    "product_id": item.get("product_id"),
                    "name": item.get("name"),
                    "brand": item.get("brand"),
                    "category": item.get("category"),
                    "subcategory": item.get("subcategory"),
                    "query": item.get("query"),
                    "status": item.get("top_candidate_status"),
                    "confidence": int(top.get("confidence", 0)) if top else 0,
                    "domain": top.get("domain"),
                    "page_url": top.get("page_url"),
                    "image_url": top.get("image_url"),
                    "reasons": ",".join(top.get("reasons", [])) if top else "",
                    "penalties": ",".join(top.get("penalties", [])) if top else "",
                    "error": top.get("error"),
                }
            )


if __name__ == "__main__":
    raise SystemExit(main())
