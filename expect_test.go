package sweet

import (
	//"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

const testTimeout = 10 * time.Microsecond

func TestExpect(t *testing.T) {
	c := make(chan string, 3)
	testString := "testString1\n"
	c <- testString
	testString = "testString2\n"
	c <- testString
	testString = "testString3\n"
	c <- testString
	result := make(chan string)
	go func() { // time this out incase expect never returns
		err := expect("2", c)
		if err != nil {
			t.Errorf("Error running expect: %s", err.Error())
			result <- err.Error()
		}
		s := <-c
		result <- s
	}()
	select {
	case remainder := <-result:
		if remainder != "testString3\n" {
			t.Errorf("Expect stopped at the wrong place!")
		}
	case <-time.After(testTimeout):
		t.Errorf("Expect call timed out - must have never matched.")
	}
}

func TestExpectMulti(t *testing.T) {
	c := make(chan string, 3)
	testString := "testString1\n"
	c <- testString
	testString = "testString2\n"
	c <- testString
	testString = "testString3\n"
	c <- testString
	result := make(chan string)
	go func() { // time this out incase expect never returns
		matched, err := expectMulti([]string{"2", "Z", "3"}, c)
		if err != nil {
			t.Errorf("Error running expectMulti: %s", err.Error())
			result <- err.Error()
		}
		result <- matched
	}()
	select {
	case matched := <-result:
		if matched != "2" {
			t.Errorf("ExpectMulti matched the wrong string: %s", matched)
		}
	case <-time.After(testTimeout):
		t.Errorf("ExpectMulti call timed out - must have never matched.")
	}
}

func TestExpectSave(t *testing.T) {
	c := make(chan string, 3)
	testString := "testString1\n"
	c <- testString
	testString = "testString2\n"
	c <- testString
	testString = "testString3\n"
	c <- testString
	result := make(chan string)
	go func() { // time this out incase expect never returns
		saved, err := expectSave("2", c)
		if err != nil {
			t.Errorf("Error running expect: %s", err.Error())
			result <- err.Error()
		}
		result <- saved
	}()
	select {
	case saved := <-result:
		if saved != "testString1\ntestString" {
			t.Errorf("ExpectSave returned the wrong string: %s", saved)
		}
	case <-time.After(testTimeout):
		t.Errorf("ExpectSave call timed out - must have never matched.")
	}
}

func TestTimeoutSave(t *testing.T) {
	c := make(chan string)
	result := make(chan string)
	testTimeout := 1 * time.Millisecond
	go func() { // time this out incase expect never returns
		testString := "testString1\n"
		c <- testString
		time.Sleep(testTimeout - (1 * time.Millisecond))
		testString = "testString2\n"
		c <- testString
		time.Sleep(testTimeout + (1 * time.Millisecond))
		testString = "testString3\n"
		c <- testString
	}()
	go func() { // time this out incase expect never returns
		saved, err := expectSaveTimeout("#", c, testTimeout)
		if err != nil {
			t.Errorf("Error running TimeoutSave: %s", err.Error())
			result <- err.Error()
		}
		result <- saved
	}()
	select {
	case saved := <-result:
		if saved != "testString1\ntestString2\n" {
			t.Errorf("TimeoutSave returned the wrong string: %s", saved)
		}
	case <-time.After(testTimeout * 3):
		t.Errorf("TimeoutSave call timed out - must have never matched.")
	}
}

func TestReadChunk(t *testing.T) {
	// single line test
	createTestFileSH := "echo teststring1 > _sweet_test_readstring"
	err := exec.Command("sh", "-c", createTestFileSH).Run()
	if err != nil {
		t.Errorf("Unable to create test file: %s", err.Error())
		return
	}
	f, err := os.Open("_sweet_test_readstring")
	if err != nil {
		t.Errorf("Unable to open test file: %s", err.Error())
		return
	}
	s, err := readChunk(f)
	if err != nil {
		t.Errorf("Error reading string: %s", err.Error())
		return
	}
	if s != "teststring1\n" {
		t.Errorf("Got the wrong string back: %s", s)
	}
	f.Close()

	// multi-single line test
	createTestFileSH = "echo teststring1 > _sweet_test_readstring; echo teststring2 >> _sweet_test_readstring;"
	err = exec.Command("sh", "-c", createTestFileSH).Run()
	if err != nil {
		t.Errorf("Unable to update test file: %s", err.Error())
		return
	}
	f, err = os.Open("_sweet_test_readstring")
	if err != nil {
		t.Errorf("Unable to open test file: %s", err.Error())
		return
	}
	s, err = readChunk(f)
	if err != nil {
		t.Errorf("Error reading string: %s", err.Error())
		return
	}
	if s != "teststring1\nteststring2\n" {
		t.Errorf("Got the wrong multi-line string back: %s", s)
	}
	f.Close()

	// test file > chunk size (255):
	createTestFileSH = "echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa > _sweet_test_readstring"
	err = exec.Command("sh", "-c", createTestFileSH).Run()
	if err != nil {
		t.Errorf("Unable to update test file: %s", err.Error())
		return
	}
	f, err = os.Open("_sweet_test_readstring")
	if err != nil {
		t.Errorf("Unable to open test file: %s", err.Error())
		return
	}
	s, err = readChunk(f)
	if err != nil {
		t.Errorf("Error reading string: %s", err.Error())
		return
	}
	if len(s) != 255 {
		t.Errorf("Got the wrong length back from a big file: %d", len(s))
	}
	s, err = readChunk(f)
	if err != nil {
		t.Errorf("Error reading string: %s", err.Error())
		return
	}
	if len(s) != 2 {
		t.Errorf("Got the wrong length back from a big file: %d", len(s))
	}
	if s != "a\n" {
		t.Errorf("Got the wrong string back from a big file: %s", s)
	}
	f.Close()

	// cleanup
	err = exec.Command("rm", "_sweet_test_readstring").Run()
	if err != nil {
		t.Errorf("Unable to remove test file: %s", err.Error())
	}
}
