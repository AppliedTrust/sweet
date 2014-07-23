package sweet

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestBindata(t *testing.T) {
	directories := []string{"tmpl", "static"}
	files := map[string]bool{}

	// finding all bindata source files
	for _, dir := range directories {
		dirFiles, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Errorf("Can't find bindata source directory: %s", dir)
		}
		for _, f := range dirFiles {
			files[dir+string(os.PathSeparator)+f.Name()] = true
		}
	}

	// checking dashboard template file
	filename := "tmpl/index.html"
	if _, exists := files[filename]; !exists {
		t.Errorf("Critical bindata source file %s is missing", filename)
	}
	asset, err := Asset(filename)
	if err != nil {
		t.Errorf("Bindata is missing a critical file: %s", filename)
	}
	if !strings.Contains(string(asset), "Sweet status dashboard") {
		t.Errorf("%s is missing expected HTML meta tag", filename)
	}

	// checking each source file is present in bindata
	for filename = range files {
		asset, err := Asset(filename)
		if err != nil {
			t.Errorf("Bindata is missing a file: %s", filename)
		}
		if len(asset) < 1 {
			t.Errorf("Zero-length bindata file: %s", filename)
		}
	}

}
