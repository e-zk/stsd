.POSIX: 
.SUFFIXES:
.PHONY: clean

stsd: main.go
	go build -o stsd main.go

clean:
	-rm -f stsd
