VERSION=$(shell bin/sweet64 --version)

####
linux32: bindata
	GOOS=linux GOARCH=386 go build -o bin/sweet32 cmd/main.go

all: bindata binaries packages

binaries: bindata linux32 linux64 darwin64

bindata:
	cd frontend && ember build --environment=production && cd ..
	GOOS=linux GOARCH=amd64 go-bindata -pkg="sweet" frontend/dist/...

linux64: bindata
	GOOS=linux GOARCH=amd64 go build -o bin/sweet64 cmd/main.go

darwin64: bindata
	GOOS=darwin GOARCH=amd64 go build -o bin/sweet-osx cmd/main.go

####
deps:
	go get github.com/vaughan0/go-ini github.com/docopt/docopt-go github.com/kballard/go-shellquote github.com/kr/pty github.com/mgutz/ansi
	go get github.com/gorilla/handlers github.com/gorilla/mux github.com/gorilla/websocket github.com/goji/httpauth

####
packages: rpm32 deb32 rpm64 deb64

rpm32:
	rm -rf build/rpm
	mkdir -p build/rpm/sweet/usr/local/bin
	cp bin/sweet32 build/rpm/sweet/usr/local/bin/sweet
	fpm -s dir -t rpm -n sweet -a i386 --epoch 0 -v $(VERSION) -C build/rpm/sweet .
	mv sweet-$(VERSION)-1.i386.rpm bin/

rpm64:
	rm -rf build/rpm
	mkdir -p build/rpm/sweet/usr/local/bin
	cp bin/sweet64 build/rpm/sweet/usr/local/bin/sweet
	fpm -s dir -t rpm -n sweet -a x86_64 --epoch 0 -v $(VERSION) -C build/rpm/sweet .
	mv sweet-$(VERSION)-1.x86_64.rpm bin/

deb32:
	rm -rf build/deb
	mkdir -p build/deb/sweet/usr/local/bin
	cp bin/sweet32 build/deb/sweet/usr/local/bin/sweet
	fpm -s dir -t deb -n sweet -a i386 -v $(VERSION) -C build/deb/sweet .
	mv sweet_$(VERSION)_i386.deb bin/

deb64:
	rm -rf build/deb
	mkdir -p build/deb/sweet/usr/local/bin
	cp bin/sweet64 build/deb/sweet/usr/local/bin/sweet
	fpm -s dir -t deb -n sweet -a amd64 -v $(VERSION) -C build/deb/sweet .
	mv sweet_$(VERSION)_amd64.deb bin/

#### 
release:
	github-release release --user appliedtrust --repo sweet --tag $(VERSION) \
		--name "Sweet $(VERSION)" \
		--description "Network device configuration backups and change alerts for the 21st century." \
	github-release upload --user appliedtrust --repo sweet --tag $(VERSION) \
		--name "sweet-linux-32" \
		--file bin/sweet32
	github-release upload --user appliedtrust --repo sweet --tag $(VERSION) \
		--name "sweet-linux-64" \
		--file bin/sweet64
	github-release upload --user appliedtrust --repo sweet --tag $(VERSION) \
		--name "sweet-osx" \
		--file bin/sweet-osx
	github-release upload --user appliedtrust --repo sweet --tag $(VERSION) \
		--name bin/sweet_$(VERSION)_i386.deb \
		--file bin/sweet_$(VERSION)_i386.deb
	github-release upload --user appliedtrust --repo sweet --tag $(VERSION) \
		--name bin/sweet-$(VERSION)-1.i386.rpm \
		--file bin/sweet-$(VERSION)-1.i386.rpm

