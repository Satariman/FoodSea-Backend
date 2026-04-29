.PHONY: install proto test run probe-weights

PYTHON ?= python3
VENV_PY := .venv/bin/python
ifeq ($(wildcard $(VENV_PY)), $(VENV_PY))
PYTHON := $(VENV_PY)
endif

PIP := $(PYTHON) -m pip

PROTO_DIR = ../../proto
OUT_DIR = src/proto

install:
	$(PIP) install -U pip "setuptools>=68,<82" wheel
	$(PIP) install -r requirements.txt
	$(PIP) install -e .

proto:
	$(PYTHON) -m grpc_tools.protoc \
		-I$(PROTO_DIR)/ml \
		-I$(PROTO_DIR)/core \
		--python_out=$(OUT_DIR) \
		--grpc_python_out=$(OUT_DIR) \
		$(PROTO_DIR)/ml/analogs.proto \
		$(PROTO_DIR)/core/catalog.proto
	$(PYTHON) -c 'from pathlib import Path; \
paths=[Path("src/proto/analogs_pb2_grpc.py"), Path("src/proto/catalog_pb2_grpc.py")]; \
repls=[("import analogs_pb2 as analogs__pb2","from . import analogs_pb2 as analogs__pb2"),("import catalog_pb2 as catalog__pb2","from . import catalog_pb2 as catalog__pb2")]; \
[p.write_text(__import__("functools").reduce(lambda acc, r: acc.replace(*r), repls, p.read_text(encoding="utf-8")), encoding="utf-8") for p in paths]'

test:
	$(PYTHON) -m pytest tests/ -v

run:
	$(PYTHON) -m src.main

probe-weights:
	$(PYTHON) -m src.weight_probe $(ARGS)
