.PHONY: build run clean

build:
	go build -o portsMaster .

run: build
	./portsMaster

clean:
	rm -rf ./public ./.cache ./content ./data ./portsMaster
