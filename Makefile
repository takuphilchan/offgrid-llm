.PHONY: build test test-go test-web test-desktop test-computer test-python check-docs lint

# Contributor shortcuts only. Installation, deployment, and live-service tests
# require explicit commands and are intentionally not targets here.
build:
	mkdir -p build/dev
	go build -o build/dev/offgrid ./cmd/offgrid
	npm --prefix apps/web run build

test: test-go test-web test-desktop test-computer test-python check-docs

test-go:
	go test ./...

test-web:
	npm --prefix apps/web run api:check
	npm --prefix apps/web run check

test-desktop:
	npm --prefix apps/desktop test

test-computer:
	npm --prefix apps/computer test

test-python:
	PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests/test_client.py
	python3 -m pytest -q runtimes/python/tests

check-docs:
	node --test scripts/maintenance/check-docs.test.mjs
	node scripts/maintenance/check-docs.mjs

lint:
	go vet ./...
	node scripts/maintenance/check-repository-hygiene.mjs
	node scripts/maintenance/check-application-layout.mjs
	node scripts/maintenance/check-developer-layout.mjs
	node --test scripts/maintenance/check-source-size.test.mjs
	node scripts/maintenance/check-source-size.mjs
