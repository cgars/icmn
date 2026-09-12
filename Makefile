.PHONY: run test integration fmt vet check paper diagrams diagram-png verify-diagram-png

DRAWIO_IMAGE := rlespinasse/drawio-desktop-headless@sha256:4e6cf6152c67259a597ffdd13cc5db82014fd3e37328a71ea294b101257601f9
DIAGRAM_SOURCE := docs/architecture/icmn-architecture.drawio
DIAGRAM_SVG := docs/architecture/icmn-architecture.svg
DIAGRAM_PNG := build/architecture/icmn-architecture.png

run:
	go run ./cmd/icmn

test:
	go test ./...

integration:
	test -n "$$ICMN_TEST_DATABASE_URL"
	go test -race ./internal/postgres

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

vet:
	go vet ./...

check:
	test -z "$$(gofmt -l $$(find . -name '*.go' -type f))"
	go vet ./...
	go test -race ./...

paper:
	cd paper && pdflatex -interaction=nonstopmode -halt-on-error icmn.tex

diagrams:
	command -v drawio >/dev/null
	drawio --export --format svg --output $(DIAGRAM_SVG) $(DIAGRAM_SOURCE)

diagram-png:
	command -v docker >/dev/null
	mkdir -p $$(dirname $(DIAGRAM_PNG))
	docker run --rm --volume "$$(pwd):/data" --workdir /data $(DRAWIO_IMAGE) --export --format png --scale 2 --output $(DIAGRAM_PNG) $(DIAGRAM_SOURCE)

verify-diagram-png:
	python3 scripts/verify_diagram_png.py $(DIAGRAM_PNG)
