.PHONY: dev
dev: sync build-go

.PHONY: sync
sync:
	uv sync
	ln -fs "$$(realpath -s --relative-to="$$(realpath .venv/bin)" redbot-update)" .venv/bin/redbot-update

.PHONY: build-all
build-all: build-go build-python

.PHONY: build-go
build-go:
	CGO_ENABLED=1 go build ./...
	CGO_ENABLED=1 go build ./go/cmd/redbot-update

.PHONY: build-python
build-python: build-sdist build-wheels

.PHONY: build-sdist
build-sdist:
	uv build --sdist

.PHONY: build-wheels
build-wheels:
	GOOS=linux GOARCH=amd64 PLATFORM_TAG=py3-none-manylinux_2_17_x86_64 uv build --wheel
	GOOS=linux GOARCH=amd64 PLATFORM_TAG=py3-none-musllinux_1_2_x86_64 uv build --wheel
	GOOS=linux GOARCH=arm64 PLATFORM_TAG=py3-none-manylinux_2_17_aarch64 uv build --wheel
	GOOS=linux GOARCH=arm64 PLATFORM_TAG=py3-none-musllinux_1_2_aarch64 uv build --wheel
	GOOS=darwin GOARCH=amd64 PLATFORM_TAG=py3-none-macosx_10_12_x86_64 uv build --wheel
	GOOS=darwin GOARCH=arm64 PLATFORM_TAG=py3-none-macosx_11_0_arm64 uv build --wheel
	GOOS=windows GOARCH=amd64 PLATFORM_TAG=py3-none-win_amd64 uv build --wheel
	GOOS=windows GOARCH=arm64 PLATFORM_TAG=py3-none-win_arm64 uv build --wheel

.PHONY: fmt format reformat
fmt format reformat: fmt-go fmt-python

.PHONY: fmt-go format-go reformat-go
fmt-go format-go reformat-go:
	go fmt ./...

.PHONY: fmt-python format-python reformat-python
fmt-python format-python reformat-python:
	uv run ruff check --fix --select I --show-fixes
	uv run ruff format
