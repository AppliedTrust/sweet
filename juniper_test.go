package sweet

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestJunOSGood(t *testing.T) {
	d := new(DeviceConfig)
	d.Config = make(map[string]string)
	d.Method = "junos"
	d.Timeout = 10 * time.Second

	if os.Getenv("SWEET_TEST_JUNOS_HOST") == "" {
		t.Error("Test requries SWEET_TEST_JUNOS_HOST environment variable")
		return
	}
	if os.Getenv("SWEET_TEST_JUNOS_USER") == "" {
		t.Error("Test requries SWEET_TEST_JUNOS_USER environment variable")
		return
	}
	if os.Getenv("SWEET_TEST_JUNOS_PASS") == "" {
		t.Error("Test requries SWEET_TEST_JUNOS_PASS environment variable")
		return
	}

	d.Hostname = os.Getenv("SWEET_TEST_JUNOS_HOST")
	d.Config["user"] = os.Getenv("SWEET_TEST_JUNOS_USER")
	d.Config["pass"] = os.Getenv("SWEET_TEST_JUNOS_PASS")

	d.Target = d.Hostname

	s := CollectJunOS(*d)
	if !strings.Contains(s["config"], "version ") {
		t.Errorf("Config missing version line")
	}
	if !strings.Contains(s["config"], "security") {
		t.Errorf("Config missing security line close to end")
	}

}
