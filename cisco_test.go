package sweet

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestCiscoGood(t *testing.T) {
	d := new(DeviceAccess)
	d.Config = make(map[string]string)
	d.Method = "cisco"
	d.Timeout = 10 * time.Second

	if os.Getenv("SWEET_TEST_HOST") == "" {
		t.Error("Test requries SWEET_TEST_HOST environment variable")
		return
	}
	if os.Getenv("SWEET_TEST_USER") == "" {
		t.Error("Test requries SWEET_TEST_USER environment variable")
		return
	}
	if os.Getenv("SWEET_TEST_PASS") == "" {
		t.Error("Test requries SWEET_TEST_PASS environment variable")
		return
	}

	d.Hostname = os.Getenv("SWEET_TEST_HOST")
	d.Config["user"] = os.Getenv("SWEET_TEST_USER")
	d.Config["pass"] = os.Getenv("SWEET_TEST_PASS")

	d.Target = d.Hostname

	s := CollectCisco(*d)
	if !strings.Contains(s["config"], "aaa authorization commands") {
		t.Errorf("Config missing aaa line")
	}
	if !strings.Contains(s["config"], "ntp access-group peer") {
		t.Errorf("Config missing ntp line close to end")
	}

}
