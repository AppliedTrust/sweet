package sweet

import (
	"testing"
	"time"
)

func TestErrorcache(t *testing.T) {
	ec, err := RunErrorCache()
	if err != nil {
		t.Errorf("Error running errorCache: %s", err.Error())
		return
	}

	done := make(chan bool)
	go func() {
		response := make(chan string)
		ec.Requests <- &ErrorCacheRequest{Hostname: "iDoNotExist", Response: response}
		cache := <-response
		if len(cache) > 0 {
			t.Errorf("Got a cache entry where I expected an empty string: %s", cache)
		}

		ec.Updates <- &ErrorCacheUpdate{Hostname: "iDoNotExist", ErrorMessage: "thisIsOnlyATest"}
		time.Sleep(1 * time.Millisecond)
		ec.Requests <- &ErrorCacheRequest{Hostname: "iDoNotExist", Response: response}
		cache = <-response
		if cache != "thisIsOnlyATest" {
			t.Errorf("Cache entry I just added is missing!")
		}

		ec.Updates <- &ErrorCacheUpdate{Hostname: "iDoNotExist", ErrorMessage: ""}
		time.Sleep(1 * time.Millisecond)
		ec.Requests <- &ErrorCacheRequest{Hostname: "iDoNotExist", Response: response}
		cache = <-response
		if len(cache) > 0 {
			t.Errorf("Cache entry I deleted is still there!")
		}

		done <- true
	}()

	select {
	case <-done:
	case <-time.After(4 * time.Millisecond):
		t.Errorf("ErrorCache tests timed out")
	}
}
