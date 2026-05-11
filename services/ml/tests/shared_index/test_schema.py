import numpy as np

from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow


def test_shared_schema_dataclasses_construct():
    meta = SharedIndexMeta(
        product_id="p-1",
        name="Milk",
        brand_name="Brand",
        category_name="Dairy",
        subcategory_name="Milk",
        image_url="https://example.com/milk.png",
    )
    profile = SharedIndexProfile(
        provider="gemini",
        model="text-embedding-004",
        dimensions=3,
        index_mode="multi_channel",
        build_weights={"name": 1.0},
    )
    row = SharedIndexRow(meta=meta, channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)})

    assert row.meta.product_id == "p-1"
    assert profile.dimensions == 3
    assert row.channels["name"].shape == (3,)
