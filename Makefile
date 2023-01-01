.PHONY: build dev

build:
	mkdir -p dist
	go build -o dist/bigben-website .

dev: build
	./dist/bigben-website
