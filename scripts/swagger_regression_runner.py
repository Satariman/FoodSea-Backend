#!/usr/bin/env python3
"""Run Swagger-style regression checks across core/optimization/ordering APIs."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import sys
import textwrap
import urllib.error
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any


def _shorten(value: str, limit: int = 220) -> str:
    if len(value) <= limit:
        return value
    return value[: limit - 3] + "..."


@dataclass
class HTTPResult:
    method: str
    url: str
    status: int | None
    body_text: str
    body_json: Any | None
    error: str | None = None


@dataclass
class StepResult:
    step_id: str
    scenario: str
    method: str
    endpoint: str
    expected: str
    actual_status: str
    result: str
    bug_id: str
    note: str
    response_excerpt: str


class Runner:
    def __init__(
        self,
        core_base: str,
        optimization_base: str,
        ordering_base: str,
        password: str,
        timeout: float,
    ) -> None:
        self.core_base = core_base.rstrip("/")
        self.optimization_base = optimization_base.rstrip("/")
        self.ordering_base = ordering_base.rstrip("/")
        self.password = password
        self.timeout = timeout

        self.steps: list[StepResult] = []
        self.token: str | None = None
        self.email: str | None = None
        self.product_id: str | None = None
        self.optimization_result_id: str | None = None
        self.order_id: str | None = None

    def http(
        self,
        method: str,
        url: str,
        token: str | None = None,
        json_body: dict[str, Any] | None = None,
    ) -> HTTPResult:
        payload = None
        headers = {"Accept": "application/json"}
        if json_body is not None:
            payload = json.dumps(json_body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = f"Bearer {token}"

        req = urllib.request.Request(url=url, method=method, headers=headers, data=payload)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                body_bytes = resp.read()
                status = resp.getcode()
                body_text = body_bytes.decode("utf-8", errors="replace")
                body_json = _parse_json(body_text)
                return HTTPResult(method, url, status, body_text, body_json)
        except urllib.error.HTTPError as err:
            body_text = err.read().decode("utf-8", errors="replace")
            body_json = _parse_json(body_text)
            return HTTPResult(method, url, err.code, body_text, body_json, error=str(err))
        except Exception as err:  # pragma: no cover - network/runtime guard
            return HTTPResult(method, url, None, "", None, error=str(err))

    def add_step(
        self,
        *,
        step_id: str,
        scenario: str,
        method: str,
        endpoint: str,
        expected_statuses: set[int],
        response: HTTPResult | None = None,
        skip_reason: str | None = None,
    ) -> None:
        expected = ", ".join(str(code) for code in sorted(expected_statuses))

        if skip_reason is not None:
            self.steps.append(
                StepResult(
                    step_id=step_id,
                    scenario=scenario,
                    method=method,
                    endpoint=endpoint,
                    expected=expected,
                    actual_status="-",
                    result="SKIP",
                    bug_id="-",
                    note=skip_reason,
                    response_excerpt="-",
                )
            )
            return

        if response is None:
            raise ValueError("response must be provided when step is not skipped")

        if response.status is None:
            result = "FAIL"
            actual_status = "NO_RESPONSE"
            note = response.error or "request failed before receiving response"
        else:
            is_pass = response.status in expected_statuses
            result = "PASS" if is_pass else "FAIL"
            actual_status = str(response.status)
            note = response.error or ""

        raw_body = response.body_text.strip()
        if not raw_body and response.error:
            raw_body = response.error

        self.steps.append(
            StepResult(
                step_id=step_id,
                scenario=scenario,
                method=method,
                endpoint=endpoint,
                expected=expected,
                actual_status=actual_status,
                result=result,
                bug_id="" if result == "PASS" else "TODO",
                note=_shorten(note or "-"),
                response_excerpt=_shorten(raw_body or "-"),
            )
        )

    def run(self) -> list[StepResult]:
        self._check_health()
        self._auth_flow()
        self._core_flow()
        self._optimization_and_ordering_flow()
        return self.steps

    def _check_health(self) -> None:
        targets = [
            ("SMOKE-01", "Core health", "GET", f"{self.core_base}/health"),
            ("SMOKE-02", "Optimization health", "GET", f"{self.optimization_base}/health"),
            ("SMOKE-03", "Ordering health", "GET", f"{self.ordering_base}/health"),
        ]
        for step_id, scenario, method, url in targets:
            resp = self.http(method, url)
            self.add_step(
                step_id=step_id,
                scenario=scenario,
                method=method,
                endpoint=url,
                expected_statuses={200},
                response=resp,
            )

    def _auth_flow(self) -> None:
        self.email = f"swagger.auto.{uuid.uuid4().hex[:10]}@example.com"
        register_payload = {"email": self.email, "password": self.password}

        register_resp = self.http("POST", f"{self.core_base}/api/v1/auth/register", json_body=register_payload)
        self.add_step(
            step_id="CORE-AUTH-01",
            scenario="Register user",
            method="POST",
            endpoint="/api/v1/auth/register",
            expected_statuses={200, 201},
            response=register_resp,
        )

        login_resp = self.http("POST", f"{self.core_base}/api/v1/auth/login", json_body=register_payload)
        self.add_step(
            step_id="CORE-AUTH-02",
            scenario="Login with valid credentials",
            method="POST",
            endpoint="/api/v1/auth/login",
            expected_statuses={200},
            response=login_resp,
        )

        self.token = _safe_get(login_resp.body_json, "data", "access_token")

        bad_login_resp = self.http(
            "POST",
            f"{self.core_base}/api/v1/auth/login",
            json_body={"email": self.email, "password": f"{self.password}bad"},
        )
        self.add_step(
            step_id="CORE-AUTH-03",
            scenario="Login with wrong password",
            method="POST",
            endpoint="/api/v1/auth/login",
            expected_statuses={400, 401},
            response=bad_login_resp,
        )

        me_unauth_resp = self.http("GET", f"{self.core_base}/api/v1/users/me")
        self.add_step(
            step_id="CORE-AUTH-04",
            scenario="Profile without token",
            method="GET",
            endpoint="/api/v1/users/me",
            expected_statuses={401},
            response=me_unauth_resp,
        )

        if not self.token:
            self.add_step(
                step_id="CORE-AUTH-05",
                scenario="Profile with token",
                method="GET",
                endpoint="/api/v1/users/me",
                expected_statuses={200},
                skip_reason="No access_token from login response",
            )
            return

        me_auth_resp = self.http("GET", f"{self.core_base}/api/v1/users/me", token=self.token)
        self.add_step(
            step_id="CORE-AUTH-05",
            scenario="Profile with token",
            method="GET",
            endpoint="/api/v1/users/me",
            expected_statuses={200},
            response=me_auth_resp,
        )

    def _core_flow(self) -> None:
        products_resp = self.http("GET", f"{self.core_base}/api/v1/products")
        self.add_step(
            step_id="CORE-01",
            scenario="List products",
            method="GET",
            endpoint="/api/v1/products",
            expected_statuses={200},
            response=products_resp,
        )
        self.product_id = _extract_first_product_id(products_resp.body_json)

        search_resp = self.http("GET", f"{self.core_base}/api/v1/search?q=milk")
        self.add_step(
            step_id="CORE-02",
            scenario="Search products",
            method="GET",
            endpoint="/api/v1/search?q=milk",
            expected_statuses={200},
            response=search_resp,
        )

        stores_resp = self.http("GET", f"{self.core_base}/api/v1/stores")
        self.add_step(
            step_id="CORE-03",
            scenario="List stores",
            method="GET",
            endpoint="/api/v1/stores",
            expected_statuses={200},
            response=stores_resp,
        )

        barcode_resp = self.http("GET", f"{self.core_base}/api/v1/barcode/00000000")
        self.add_step(
            step_id="CORE-04",
            scenario="Barcode lookup for non-existing code",
            method="GET",
            endpoint="/api/v1/barcode/00000000",
            expected_statuses={404},
            response=barcode_resp,
        )

        if not self.token:
            for step_id, scenario, method, endpoint, expected in [
                ("CORE-05", "Get cart", "GET", "/api/v1/cart", {200}),
                ("CORE-06", "Add cart item with invalid quantity", "POST", "/api/v1/cart/items", {400}),
                ("CORE-07", "Add cart item", "POST", "/api/v1/cart/items", {200, 201}),
                ("CORE-08", "Get cart after add", "GET", "/api/v1/cart", {200}),
                ("CORE-09", "Delete cart item", "DELETE", "/api/v1/cart/items/{product_id}", {204}),
                ("CORE-10", "Add cart item again", "POST", "/api/v1/cart/items", {200, 201}),
            ]:
                self.add_step(
                    step_id=step_id,
                    scenario=scenario,
                    method=method,
                    endpoint=endpoint,
                    expected_statuses=expected,
                    skip_reason="No access_token from auth flow",
                )
            return

        cart_resp = self.http("GET", f"{self.core_base}/api/v1/cart", token=self.token)
        self.add_step(
            step_id="CORE-05",
            scenario="Get cart",
            method="GET",
            endpoint="/api/v1/cart",
            expected_statuses={200},
            response=cart_resp,
        )

        invalid_add_resp = self.http(
            "POST",
            f"{self.core_base}/api/v1/cart/items",
            token=self.token,
            json_body={"product_id": str(uuid.uuid4()), "quantity": 0},
        )
        self.add_step(
            step_id="CORE-06",
            scenario="Add cart item with invalid quantity",
            method="POST",
            endpoint="/api/v1/cart/items",
            expected_statuses={400},
            response=invalid_add_resp,
        )

        if not self.product_id:
            for step_id, scenario, method, endpoint, expected in [
                ("CORE-07", "Add cart item", "POST", "/api/v1/cart/items", {200, 201}),
                ("CORE-08", "Get cart after add", "GET", "/api/v1/cart", {200}),
                ("CORE-09", "Delete cart item", "DELETE", "/api/v1/cart/items/{product_id}", {204}),
                ("CORE-10", "Add cart item again", "POST", "/api/v1/cart/items", {200, 201}),
            ]:
                self.add_step(
                    step_id=step_id,
                    scenario=scenario,
                    method=method,
                    endpoint=endpoint,
                    expected_statuses=expected,
                    skip_reason="No product_id from /api/v1/products response",
                )
            return

        add_resp = self.http(
            "POST",
            f"{self.core_base}/api/v1/cart/items",
            token=self.token,
            json_body={"product_id": self.product_id, "quantity": 2},
        )
        self.add_step(
            step_id="CORE-07",
            scenario="Add cart item",
            method="POST",
            endpoint="/api/v1/cart/items",
            expected_statuses={200, 201},
            response=add_resp,
        )

        cart_after_add_resp = self.http("GET", f"{self.core_base}/api/v1/cart", token=self.token)
        self.add_step(
            step_id="CORE-08",
            scenario="Get cart after add",
            method="GET",
            endpoint="/api/v1/cart",
            expected_statuses={200},
            response=cart_after_add_resp,
        )

        delete_resp = self.http(
            "DELETE",
            f"{self.core_base}/api/v1/cart/items/{self.product_id}",
            token=self.token,
        )
        self.add_step(
            step_id="CORE-09",
            scenario="Delete cart item",
            method="DELETE",
            endpoint="/api/v1/cart/items/{product_id}",
            expected_statuses={204},
            response=delete_resp,
        )

        add_again_resp = self.http(
            "POST",
            f"{self.core_base}/api/v1/cart/items",
            token=self.token,
            json_body={"product_id": self.product_id, "quantity": 1},
        )
        self.add_step(
            step_id="CORE-10",
            scenario="Add cart item again",
            method="POST",
            endpoint="/api/v1/cart/items",
            expected_statuses={200, 201},
            response=add_again_resp,
        )

    def _optimization_and_ordering_flow(self) -> None:
        no_auth_order_resp = self.http(
            "POST",
            f"{self.ordering_base}/api/v1/orders",
            json_body={"optimization_result_id": str(uuid.uuid4())},
        )
        self.add_step(
            step_id="ORDER-NEG-01",
            scenario="Create order without token",
            method="POST",
            endpoint="/api/v1/orders",
            expected_statuses={401},
            response=no_auth_order_resp,
        )

        if not self.token:
            for step_id, scenario, method, endpoint, expected in [
                ("OPT-01", "Run optimization", "POST", "/api/v1/optimize", {200}),
                ("ORDER-01", "Create order from optimization result", "POST", "/api/v1/orders", {201}),
                ("ORDER-02", "List orders", "GET", "/api/v1/orders", {200}),
                ("ORDER-03", "Get order by id", "GET", "/api/v1/orders/{id}", {200}),
                ("ORDER-04", "Get saga state", "GET", "/api/v1/orders/{id}/saga", {200}),
                ("ORDER-05", "Invalid order status transition", "PATCH", "/api/v1/orders/{id}/status", {400, 409}),
                ("ORDER-NEG-02", "Get random order id", "GET", "/api/v1/orders/{random}", {404}),
            ]:
                self.add_step(
                    step_id=step_id,
                    scenario=scenario,
                    method=method,
                    endpoint=endpoint,
                    expected_statuses=expected,
                    skip_reason="No access_token from auth flow",
                )
            return

        optimize_resp = self.http("POST", f"{self.optimization_base}/api/v1/optimize", token=self.token)
        self.add_step(
            step_id="OPT-01",
            scenario="Run optimization",
            method="POST",
            endpoint="/api/v1/optimize",
            expected_statuses={200},
            response=optimize_resp,
        )
        self.optimization_result_id = _safe_get(optimize_resp.body_json, "data", "id")

        if not self.optimization_result_id:
            for step_id, scenario, method, endpoint, expected in [
                ("ORDER-01", "Create order from optimization result", "POST", "/api/v1/orders", {201}),
                ("ORDER-02", "List orders", "GET", "/api/v1/orders", {200}),
                ("ORDER-03", "Get order by id", "GET", "/api/v1/orders/{id}", {200}),
                ("ORDER-04", "Get saga state", "GET", "/api/v1/orders/{id}/saga", {200}),
                ("ORDER-05", "Invalid order status transition", "PATCH", "/api/v1/orders/{id}/status", {400, 409}),
            ]:
                self.add_step(
                    step_id=step_id,
                    scenario=scenario,
                    method=method,
                    endpoint=endpoint,
                    expected_statuses=expected,
                    skip_reason="No optimization result id from /api/v1/optimize",
                )
        else:
            create_order_resp = self.http(
                "POST",
                f"{self.ordering_base}/api/v1/orders",
                token=self.token,
                json_body={"optimization_result_id": self.optimization_result_id},
            )
            self.add_step(
                step_id="ORDER-01",
                scenario="Create order from optimization result",
                method="POST",
                endpoint="/api/v1/orders",
                expected_statuses={201},
                response=create_order_resp,
            )
            self.order_id = _safe_get(create_order_resp.body_json, "data", "order_id")

            list_orders_resp = self.http("GET", f"{self.ordering_base}/api/v1/orders", token=self.token)
            self.add_step(
                step_id="ORDER-02",
                scenario="List orders",
                method="GET",
                endpoint="/api/v1/orders",
                expected_statuses={200},
                response=list_orders_resp,
            )

            if self.order_id:
                get_order_resp = self.http("GET", f"{self.ordering_base}/api/v1/orders/{self.order_id}", token=self.token)
                self.add_step(
                    step_id="ORDER-03",
                    scenario="Get order by id",
                    method="GET",
                    endpoint="/api/v1/orders/{id}",
                    expected_statuses={200},
                    response=get_order_resp,
                )

                saga_resp = self.http("GET", f"{self.ordering_base}/api/v1/orders/{self.order_id}/saga", token=self.token)
                self.add_step(
                    step_id="ORDER-04",
                    scenario="Get saga state",
                    method="GET",
                    endpoint="/api/v1/orders/{id}/saga",
                    expected_statuses={200},
                    response=saga_resp,
                )

                invalid_transition_resp = self.http(
                    "PATCH",
                    f"{self.ordering_base}/api/v1/orders/{self.order_id}/status",
                    token=self.token,
                    json_body={"status": "created", "comment": "negative test"},
                )
                self.add_step(
                    step_id="ORDER-05",
                    scenario="Invalid order status transition",
                    method="PATCH",
                    endpoint="/api/v1/orders/{id}/status",
                    expected_statuses={400, 409},
                    response=invalid_transition_resp,
                )
            else:
                for step_id, scenario, method, endpoint, expected in [
                    ("ORDER-03", "Get order by id", "GET", "/api/v1/orders/{id}", {200}),
                    ("ORDER-04", "Get saga state", "GET", "/api/v1/orders/{id}/saga", {200}),
                    ("ORDER-05", "Invalid order status transition", "PATCH", "/api/v1/orders/{id}/status", {400, 409}),
                ]:
                    self.add_step(
                        step_id=step_id,
                        scenario=scenario,
                        method=method,
                        endpoint=endpoint,
                        expected_statuses=expected,
                        skip_reason="No order_id from order create response",
                    )

        random_order_resp = self.http(
            "GET",
            f"{self.ordering_base}/api/v1/orders/{uuid.uuid4()}",
            token=self.token,
        )
        self.add_step(
            step_id="ORDER-NEG-02",
            scenario="Get random order id",
            method="GET",
            endpoint="/api/v1/orders/{random}",
            expected_statuses={404},
            response=random_order_resp,
        )


def _safe_get(payload: Any, *path: str) -> Any | None:
    cur = payload
    for key in path:
        if not isinstance(cur, dict) or key not in cur:
            return None
        cur = cur[key]
    return cur


def _parse_json(value: str) -> Any | None:
    value = value.strip()
    if not value:
        return None
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        return None


def _extract_first_product_id(payload: Any) -> str | None:
    data = _safe_get(payload, "data")
    if not isinstance(data, list):
        return None
    for item in data:
        if isinstance(item, dict):
            product_id = item.get("id")
            if isinstance(product_id, str) and product_id:
                return product_id
    return None


def write_report(steps: list[StepResult], markdown_path: Path, json_path: Path) -> None:
    total = len(steps)
    passed = sum(1 for s in steps if s.result == "PASS")
    failed = sum(1 for s in steps if s.result == "FAIL")
    skipped = sum(1 for s in steps if s.result == "SKIP")

    lines = [
        "# Swagger Regression Report",
        "",
        f"- Generated at: `{dt.datetime.now().isoformat(timespec='seconds')}`",
        f"- Total: `{total}`",
        f"- Passed: `{passed}`",
        f"- Failed: `{failed}`",
        f"- Skipped: `{skipped}`",
        "",
        "| Step | Scenario | Method | Endpoint | Expected | Actual | Result | Bug ID | Note | Response (excerpt) |",
        "|---|---|---|---|---|---|---|---|---|---|",
    ]

    for s in steps:
        lines.append(
            "| {step} | {scenario} | `{method}` | `{endpoint}` | {expected} | {actual} | {result} | {bug_id} | {note} | `{response}` |".format(
                step=s.step_id,
                scenario=s.scenario.replace("|", "\\|"),
                method=s.method,
                endpoint=s.endpoint.replace("|", "\\|"),
                expected=s.expected,
                actual=s.actual_status,
                result=s.result,
                bug_id=s.bug_id or "-",
                note=s.note.replace("|", "\\|"),
                response=s.response_excerpt.replace("|", "\\|").replace("\n", "\\n"),
            )
        )

    markdown_path.parent.mkdir(parents=True, exist_ok=True)
    markdown_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    json_payload = {
        "generated_at": dt.datetime.now().isoformat(timespec="seconds"),
        "summary": {"total": total, "passed": passed, "failed": failed, "skipped": skipped},
        "steps": [s.__dict__ for s in steps],
    }
    json_path.parent.mkdir(parents=True, exist_ok=True)
    json_path.write_text(json.dumps(json_payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run Swagger regression checks for FoodSea backend.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=textwrap.dedent(
            """\
            Example:
              python3 scripts/swagger_regression_runner.py
            """
        ),
    )
    parser.add_argument("--core-base-url", default="http://localhost:8081")
    parser.add_argument("--optimization-base-url", default="http://localhost:8082")
    parser.add_argument("--ordering-base-url", default="http://localhost:8083")
    parser.add_argument("--password", default="Passw0rd!123")
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument(
        "--report-prefix",
        default=f"reports/swagger-regression-{dt.datetime.now().strftime('%Y%m%d-%H%M%S')}",
        help="Output path prefix without extension; .md and .json will be added.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    runner = Runner(
        core_base=args.core_base_url,
        optimization_base=args.optimization_base_url,
        ordering_base=args.ordering_base_url,
        password=args.password,
        timeout=args.timeout,
    )
    steps = runner.run()

    base = Path(args.report_prefix)
    md_path = base.with_suffix(".md")
    json_path = base.with_suffix(".json")
    write_report(steps, md_path, json_path)

    failed = sum(1 for s in steps if s.result == "FAIL")
    skipped = sum(1 for s in steps if s.result == "SKIP")
    print(f"Report written: {md_path}")
    print(f"Report written: {json_path}")
    print(f"Summary: total={len(steps)} passed={len(steps)-failed-skipped} failed={failed} skipped={skipped}")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
