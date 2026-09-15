.PHONY: run test integration fmt vet check paper diagrams diagram-png verify-diagram-png

INKSCAPE_IMAGE := minidocks/inkscape@sha256:cd400193b794f494a48e84d385dea0e1ad5ae27ae032bd0b1383cd83b6c297d1
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
	docker run --rm --volume "$$(pwd):/data" --workdir /data $(INKSCAPE_IMAGE) $(DIAGRAM_SVG) --export-type=png --export-filename=$(DIAGRAM_PNG) --export-width=3200

verify-diagram-png:
	python3 scripts/verify_diagram_png.py $(DIAGRAM_PNG)
