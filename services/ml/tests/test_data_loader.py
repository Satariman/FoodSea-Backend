from __future__ import annotations

from src.data_loader import DataLoader
from src.proto import catalog_pb2


class _DummyChannel:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False


def test_load_products_maps_extended_fields(monkeypatch) -> None:
    response = catalog_pb2.ListProductsForMLResponse(
        products=[
            catalog_pb2.ProductFeaturesProto(
                product_id="p1",
                name="Milk",
                description="desc",
                composition="comp",
                category_id="c1",
                subcategory_id="sc1",
                brand_id="b1",
                weight="1 л",
                calories=42.0,
                protein=3.2,
                fat=2.5,
                carbohydrates=4.7,
                offers=[
                    catalog_pb2.ProductOfferBrief(store_id="s1", price_kopecks=10990),
                    catalog_pb2.ProductOfferBrief(store_id="s2", price_kopecks=9990),
                ],
                category_name="Dairy",
                subcategory_name="Milk",
                brand_name="BrandA",
                image_url="https://cdn.example/p1.jpg",
            )
        ]
    )

    class _Stub:
        def __init__(self, _channel) -> None:
            pass

        def ListProductsForML(self, _request):
            return response

    monkeypatch.setattr("src.data_loader.grpc.insecure_channel", lambda _addr: _DummyChannel())
    monkeypatch.setattr("src.data_loader.catalog_pb2_grpc.CatalogServiceStub", _Stub)

    loader = DataLoader("localhost:9091")
    products = loader.load_products()

    assert len(products) == 1
    product = products[0]
    assert product.category_name == "Dairy"
    assert product.subcategory_name == "Milk"
    assert product.brand_name == "BrandA"
    assert product.image_url == "https://cdn.example/p1.jpg"
    assert product.offers == {"s1": 10990, "s2": 9990}
    assert product.min_price_kopecks == 9990
