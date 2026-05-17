FROM python:3.11-slim AS base

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

FROM base AS index-builder

COPY requirements-runtime.txt requirements-builder.txt ./
RUN pip install --no-cache-dir --no-compile -r requirements-builder.txt

COPY src ./src
COPY scripts ./scripts
COPY data ./data

# Default command for offline index rebuild workflows.
CMD ["python", "-m", "src.shared_index.rebuild"]

FROM base AS runtime

COPY requirements-runtime.txt ./
RUN pip install --no-cache-dir --no-compile -r requirements-runtime.txt

COPY src ./src
COPY data ./data

EXPOSE 50051

CMD ["python", "-m", "src.main"]
