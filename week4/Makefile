.PHONY: build test
build:
	.venv/bin/python -m pip install --no-build-isolation .
	.venv/bin/kotlin-index --help >/dev/null
test:
	.venv/bin/python -m unittest discover -s tests -v
