.PHONY: build run generate clean

build: generate
	go build -o portsMaster main.go

generate:
	templ generate

run: build
	./portsMaster

clean:
	rm -rf ./public
	rm -rf ./.cache
	rm -f portsMaster
