package sweet

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"time"
)

// Throw out everything from channel until "until" string is matched
func expect(until string, receive chan string) error {
	_, err := expectSave(until, receive)
	return err
}

// Throw out everything from channel until one of "untilMulti" is matched, tell us which one
func expectMulti(untilMulti []string, receive chan string) (string, error) {
	all := ""
	for {
		select {
		case s, exists := <-receive:
			if !exists {
				return "", errors.New("Connection closed unexpectedly.")
			}
			all += s
			for _, until := range untilMulti {
				if strings.Contains(all, until) {
					return until, nil
				}
			}
		}
	}
	return all, nil
}

// Save everything from channel until "until" string is matched
func expectSave(until string, receive chan string) (string, error) {
	all := ""
	for !strings.Contains(all, until) {
		select {
		case s, exists := <-receive:
			if !exists {
				return "", errors.New("Connection closed unexpectedly.")
			}
			all += s
		}
	}
	all = all[:strings.Index(all, until)] // throw away anything after first until
	return all, nil
}

// Save everything from the channel with a read timeout
func expectSaveTimeout(until string, receive chan string, timeout time.Duration) (string, error) {
	all := ""
	for {
		select {
		case s, exists := <-receive:
			if !exists {
				return "", errors.New("Connection closed unexpectedly.")
			}
			all += s
			if strings.Contains(all, until) {
				return all, nil
			}
		case _ = <-time.After(timeout):
			return all, errors.New("Connection timeout.")
		}
	}
	return all, nil
}

// Read up to a full chunk from the session, removing nulls
func readChunk(pty *os.File) (string, error) {
	chunk := make([]byte, 255)
	n, err := pty.Read(chunk)
	if err != nil {
		return "", err
	} else if n < 1 {
		return "", errors.New("Read zero-length string")
	}
	chunk = bytes.Trim(chunk, "\x00")
	return string(chunk), nil
}
