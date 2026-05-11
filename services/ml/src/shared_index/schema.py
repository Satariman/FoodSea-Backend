from __future__ import annotations

from dataclasses import dataclass

import numpy as np


@dataclass(frozen=True)
class SharedIndexMeta:
    product_id: str
    name: str
    brand_name: str
    category_name: str
    subcategory_name: str
    image_url: str


@dataclass(frozen=True)
class SharedIndexProfile:
    provider: str
    model: str
    dimensions: int
    index_mode: str
    build_weights: dict[str, float]


@dataclass(frozen=True)
class SharedIndexRow:
    meta: SharedIndexMeta
    channels: dict[str, np.ndarray]
