.POSIX: 
.SUFFIXES:
.PHONY: clean

stsd: main.go setdate.go
	go build -o stsd ./...

clean:
	-rm -f stsd
