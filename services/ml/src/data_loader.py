"""gRPC data loading from core CatalogService."""

from __future__ import annotations

from dataclasses import dataclass

import grpc

from src.proto import catalog_pb2, catalog_pb2_grpc


@dataclass(slots=True)
class ProductData:
    """Flat product representation used by ML feature and index builders."""

    product_id: str
    name: str
    description: str
    composition: str
    category_id: str
    subcategory_id: str
    brand_id: str
    weight: str
    calories: float
    protein: float
    fat: float
    carbohydrates: float
    offers: dict[str, int]
    min_price_kopecks: int


class DataLoader:
    """Loads product feature rows over gRPC from core-service."""

    def __init__(self, core_grpc_addr: str) -> None:
        self.addr = core_grpc_addr

    def load_products(self) -> list[ProductData]:
        with grpc.insecure_channel(self.addr) as channel:
            stub = catalog_pb2_grpc.CatalogServiceStub(channel)
            request_cls = getattr(catalog_pb2, "ListProductsForMLRequest")
            resp = stub.ListProductsForML(request_cls())

        products: list[ProductData] = []
        for proto_product in resp.products:
            offers = {offer.store_id: offer.price_kopecks for offer in proto_product.offers}
            min_price = min(offers.values()) if offers else 0
            products.append(
                ProductData(
                    product_id=proto_product.product_id,
                    name=proto_product.name,
                    description=proto_product.description,
                    composition=proto_product.composition,
                    category_id=proto_product.category_id,
                    subcategory_id=proto_product.subcategory_id,
                    brand_id=proto_product.brand_id,
                    weight=proto_product.weight,
                    calories=proto_product.calories,
                    protein=proto_product.protein,
                    fat=proto_product.fat,
                    carbohydrates=proto_product.carbohydrates,
                    offers=offers,
                    min_price_kopecks=min_price,
                )
            )

        return products
