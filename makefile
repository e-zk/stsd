.POSIX: 
.SUFFIXES:
.PHONY: clean install uninstall

PREFIX = /usr/local

stsd: main.go setdate.go
	go build -ldflags "-w -s" -o stsd -v ./...

install: stsd
	install -c -m 0755 stsd $(PREFIX)/bin

uninstall:
	rm -f $(PREFIX)/bin/stsd

clean:
	-rm -f stsd
	go clean
