import logging
from typing import Protocol

import urllib.request

logger = logging.getLogger(__name__)


class ImageFetcher(Protocol):
    async def fetch_safe(self, url: str) -> bytes | None: ...


class HTTPImageFetcher:
    def __init__(self, timeout_sec: float = 5.0) -> None:
        self.timeout = timeout_sec

    async def fetch_safe(self, url: str) -> bytes | None:
        import asyncio
        try:
            return await asyncio.to_thread(self._fetch_blocking, url)
        except Exception as e:
            logger.warning("image fetch failed: url=%s err=%s", url, e)
            return None

    def _fetch_blocking(self, url: str) -> bytes:
        with urllib.request.urlopen(url, timeout=self.timeout) as resp:
            return resp.read()
