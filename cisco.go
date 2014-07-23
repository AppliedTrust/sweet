package sweet

import (
	"fmt"
	"time"
)

func CollectCisco(device DeviceAccess) map[string]string {
	result := make(map[string]string)
	result["err"] = ""

	c, err := newSSHCollector(device)
	if err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}

	if err := expect("assword:", c.Receive); err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}
	c.Send <- device.Config["pass"] + "\n"
	multi := []string{"#", ">", "assword:"}
	m, err := expectMulti(multi, c.Receive)
	if err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}
	if m == "assword:" { // bad pw
		result["err"] = fmt.Sprintf("%s: Bad login password.", device.Hostname)
		return result
	} else if m == ">" { // attempt enable
		c.Send <- "enable\n"
		if err := expect("assword:", c.Receive); err != nil {
			result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
			return result
		}
		c.Send <- device.Config["enable"] + "\n"
		if err := expect("#", c.Receive); err != nil {
			result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
			return result
		}
	}
	c.Send <- "terminal length 0\n"
	if err := expect("#", c.Receive); err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}
	c.Send <- "terminal pager 0\n"
	if err := expect("#", c.Receive); err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}
	c.Send <- "show conf\n"
	result["config"], err = timeoutSave(c.Receive, 500*time.Millisecond)
	if err != nil {
		result["err"] = fmt.Sprintf("%s: %s", device.Hostname, err.Error())
		return result
	}
	c.Send <- "exit\n"

	return result
}
