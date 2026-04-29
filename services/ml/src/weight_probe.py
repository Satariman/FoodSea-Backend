"""CLI tool to inspect analog quality under different feature weights."""

from __future__ import annotations

import argparse
from collections import defaultdict
from dataclasses import dataclass

import numpy as np

from src.config import Config
from src.data_loader import DataLoader, ProductData
from src.feature_builder import FeatureBuilder
from src.index import AnalogIndex


@dataclass(slots=True)
class FeatureSlices:
    text: slice
    nutrition: slice
    category: slice
    weight: slice
    price: slice


def truncate(text: str, max_len: int) -> str:
    if max_len <= 1:
        return text[:max_len]
    if len(text) <= max_len:
        return text
    return text[: max_len - 1] + "…"


def fmt_part(value: float) -> str:
    abs_v = abs(value)
    if 0 < abs_v < 1e-5:
        return f"{value:.2e}"
    return f"{value:+.6f}"


def parse_args() -> argparse.Namespace:
    config = Config()
    parser = argparse.ArgumentParser(
        description=(
            "Builds ML analog index and prints top analogs with contribution matrix "
            "for text/nutrition/category/weight/price feature groups."
        )
    )
    parser.add_argument("--core-grpc-addr", default=config.CORE_GRPC_ADDR)
    parser.add_argument("--text-model", default=config.TEXT_MODEL)
    parser.add_argument("--top-k", type=int, default=10)
    parser.add_argument("--pool-size", type=int, default=8)
    parser.add_argument(
        "--pool-product-ids",
        default="",
        help="Comma-separated product IDs to evaluate. If empty, diverse pool is auto-selected.",
    )
    parser.add_argument("--category-weight", type=float, default=config.CATEGORY_WEIGHT)
    parser.add_argument("--nutrition-weight", type=float, default=config.NUTRITION_WEIGHT)
    parser.add_argument("--text-weight", type=float, default=config.TEXT_WEIGHT)
    parser.add_argument("--price-weight", type=float, default=config.PRICE_WEIGHT)
    parser.add_argument("--price-penalty", type=float, default=config.PRICE_PENALTY)
    parser.add_argument("--min-score-threshold", type=float, default=0.0)
    parser.add_argument("--price-aware", action="store_true")
    parser.add_argument(
        "--filter-store-ids",
        default="",
        help="Comma-separated store IDs used as filter in index.query.",
    )
    return parser.parse_args()


def make_slices(total_dim: int, categories_dim: int) -> FeatureSlices:
    nutrition_dim = 4
    weight_dim = 1
    price_dim = 1
    text_dim = total_dim - nutrition_dim - categories_dim - weight_dim - price_dim
    if text_dim <= 0:
        raise ValueError(
            f"invalid feature layout: total_dim={total_dim}, category_dim={categories_dim}, "
            f"calculated text_dim={text_dim}"
        )
    return FeatureSlices(
        text=slice(0, text_dim),
        nutrition=slice(text_dim, text_dim + nutrition_dim),
        category=slice(text_dim + nutrition_dim, text_dim + nutrition_dim + categories_dim),
        weight=slice(total_dim - 2, total_dim - 1),
        price=slice(total_dim - 1, total_dim),
    )


def group_contributions(
    source_vec: np.ndarray,
    candidate_vec: np.ndarray,
    slices: FeatureSlices,
) -> dict[str, float]:
    src_norm = float(np.linalg.norm(source_vec))
    cand_norm = float(np.linalg.norm(candidate_vec))
    denom = src_norm * cand_norm
    if denom == 0.0:
        return {"text": 0.0, "nutrition": 0.0, "category": 0.0, "weight": 0.0, "price": 0.0}

    def contrib(part: slice) -> float:
        return float(np.dot(source_vec[part], candidate_vec[part]) / denom)

    return {
        "text": contrib(slices.text),
        "nutrition": contrib(slices.nutrition),
        "category": contrib(slices.category),
        "weight": contrib(slices.weight),
        "price": contrib(slices.price),
    }


def select_diverse_pool(products: list[ProductData], pool_size: int) -> list[str]:
    by_category: dict[str, list[ProductData]] = defaultdict(list)
    for product in products:
        by_category[product.category_id or "unknown"].append(product)

    for bucket in by_category.values():
        bucket.sort(key=lambda item: (item.name.lower(), item.product_id))

    categories = sorted(by_category.keys())
    selected: list[str] = []
    cursor = {cat: 0 for cat in categories}
    while len(selected) < pool_size:
        progressed = False
        for cat in categories:
            idx = cursor[cat]
            bucket = by_category[cat]
            if idx >= len(bucket):
                continue
            selected.append(bucket[idx].product_id)
            cursor[cat] += 1
            progressed = True
            if len(selected) >= pool_size:
                break
        if not progressed:
            break
    return selected


def resolve_pool_ids(
    products: list[ProductData],
    explicit_ids_raw: str,
    pool_size: int,
) -> list[str]:
    all_ids = {p.product_id for p in products}
    if explicit_ids_raw.strip():
        requested = [item.strip() for item in explicit_ids_raw.split(",") if item.strip()]
        unknown = [item for item in requested if item not in all_ids]
        if unknown:
            raise ValueError(f"unknown product IDs in --pool-product-ids: {', '.join(unknown)}")
        return requested
    return select_diverse_pool(products, pool_size)


def as_table(rows: list[list[str]], headers: list[str]) -> str:
    max_widths = {
        "candidate_name": 36,
        "candidate_id": 16,
    }
    widths = [len(h) for h in headers]
    for row in rows:
        for idx, value in enumerate(row):
            header = headers[idx]
            limited_value = truncate(value, max_widths.get(header, 10_000))
            widths[idx] = max(widths[idx], len(limited_value))

    def render_row(values: list[str]) -> str:
        cells: list[str] = []
        for idx, value in enumerate(values):
            header = headers[idx]
            limited_value = truncate(value, max_widths.get(header, 10_000))
            cells.append(limited_value.ljust(widths[idx]))
        return " | ".join(cells)

    divider = "-+-".join("-" * w for w in widths)
    lines = [render_row(headers), divider]
    lines.extend(render_row(row) for row in rows)
    return "\n".join(lines)


def run() -> None:
    args = parse_args()

    if args.top_k <= 0:
        raise ValueError("--top-k must be > 0")
    if args.pool_size <= 0:
        raise ValueError("--pool-size must be > 0")

    print(f"Loading products from core-service: {args.core_grpc_addr}")
    loader = DataLoader(args.core_grpc_addr)
    products = loader.load_products()
    if not products:
        print("No products loaded from core-service.")
        return

    print(f"Loaded products: {len(products)}")
    print(
        "Building vectors with weights: "
        f"text={args.text_weight}, category={args.category_weight}, "
        f"nutrition={args.nutrition_weight}, price={args.price_weight}, model={args.text_model}"
    )
    builder = FeatureBuilder(
        text_model_name=args.text_model,
        text_weight=args.text_weight,
        category_weight=args.category_weight,
        nutrition_weight=args.nutrition_weight,
        price_weight=args.price_weight,
    )
    vectors = builder.build(products)
    category_dims = len(builder.category_to_idx)
    slices = make_slices(vectors.shape[1], category_dims)
    raw_prices = np.array([p.min_price_kopecks for p in products], dtype=np.float32)
    normalized_price_feature = vectors[:, slices.price]
    print(
        "Price stats: "
        f"raw_min={int(raw_prices.min())}, raw_max={int(raw_prices.max())}, "
        f"feature_min={float(normalized_price_feature.min()):.6f}, "
        f"feature_max={float(normalized_price_feature.max()):.6f}"
    )
    if np.allclose(normalized_price_feature, 0.0):
        print(
            "WARNING: price feature is all zeros in this run. "
            "Likely all products have identical min_price_kopecks (or all prices are 0), "
            "so price cannot influence ranking."
        )

    index = AnalogIndex()
    index.build(
        product_ids=[p.product_id for p in products],
        names={p.product_id: p.name for p in products},
        vectors=vectors,
        offers={p.product_id: p.offers for p in products},
    )
    store_filter = {sid.strip() for sid in args.filter_store_ids.split(",") if sid.strip()} or None
    by_id = {p.product_id: p for p in products}

    pool_ids = resolve_pool_ids(products, args.pool_product_ids, args.pool_size)
    print(f"Evaluation pool size: {len(pool_ids)}")
    print(
        "Feature layout: "
        f"text={slices.text.stop - slices.text.start}, "
        f"nutrition=4, category={category_dims}, weight=1, price=1"
    )

    for pool_idx, source_id in enumerate(pool_ids, start=1):
        source = by_id[source_id]
        print("\n" + "=" * 120)
        print(f"[{pool_idx}/{len(pool_ids)}] SOURCE: {source.name} ({source.product_id})")
        print(f"  category={source.category_id} min_price={source.min_price_kopecks} kopecks")

        results = index.query(
            product_id=source_id,
            top_k=args.top_k,
            price_aware=args.price_aware,
            filter_store_ids=store_filter,
            price_penalty=args.price_penalty,
        )
        if not results:
            print("  No analogs found.")
            continue

        source_vec = vectors[index.id_to_idx[source_id]]
        table_rows: list[list[str]] = []
        for rank, (candidate_id, candidate_name, score, min_price) in enumerate(results, start=1):
            if score < args.min_score_threshold:
                continue
            candidate_vec = vectors[index.id_to_idx[candidate_id]]
            contrib = group_contributions(source_vec, candidate_vec, slices)
            contrib_total = (
                contrib["text"]
                + contrib["nutrition"]
                + contrib["category"]
                + contrib["weight"]
                + contrib["price"]
            )

            table_rows.append(
                [
                    str(rank),
                    candidate_name,
                    candidate_id,
                    str(min_price),
                    f"{score:.4f}",
                    fmt_part(contrib["text"]),
                    fmt_part(contrib["nutrition"]),
                    fmt_part(contrib["category"]),
                    fmt_part(contrib["weight"]),
                    fmt_part(contrib["price"]),
                    fmt_part(contrib_total),
                ]
            )

        if not table_rows:
            print(f"  No analogs after min-score-threshold={args.min_score_threshold}.")
            continue

        headers = [
            "rank",
            "candidate_name",
            "candidate_id",
            "min_price",
            "score",
            "text_part",
            "nutrition_part",
            "category_part",
            "weight_part",
            "price_part",
            "sum_parts",
        ]
        print(as_table(table_rows, headers))
        print("  note: score ~= sum_parts (group-wise cosine contribution decomposition).")


if __name__ == "__main__":
    run()
