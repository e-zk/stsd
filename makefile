.POSIX: 
.SUFFIXES:
.PHONY: clean

stsd:
	go build -o stsd main.go

clean:
	-rm -f stsd
