.POSIX: 
.SUFFIXES:
.PHONY: clean install uninstall

PREFIX = /usr/local

stsd: cmd/main.go cmd/setdate.go
	go build -ldflags "-linkmode external -w -s -extldflags '-static'" -o stsd ./cmd/...

install: stsd
	install -c -m 0755 stsd $(PREFIX)/bin

uninstall:
	rm -f $(PREFIX)/bin/stsd

clean:
	-rm -f stsd
	go clean
