package sweet

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.

import (
	"bytes"
	"fmt"
	"github.com/kballard/go-shellquote"
	"os/exec"
	"strings"
)

type External struct {
}

func newExternalCollector() Collector {
	return External{}
}

func (collector External) Collect(device DeviceConfig, c *Connection) (CollectionResults, error) {
	var cmd *exec.Cmd // TODO - move this to newConnection!!
	result := CollectionResults{}

	commandParts, err := shellquote.Split(device.Config["scriptPath"])
	if err != nil {
		return result, fmt.Errorf("External collection script (%s) missing: %s", device.Config["scriptPath"], err.Error())
	}
	if len(commandParts) > 1 {
		cmd = exec.Command(commandParts[0], commandParts[1:]...)
	} else {
		cmd = exec.Command(commandParts[0])
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("Error running external collection script (%s): %s", device.Config["scriptPath"], err.Error())
	}

	if err = cmd.Wait(); err != nil {
		return result, fmt.Errorf("External collection script (%s) returned an error: %s - %s", device.Config["scriptPath"], err.Error(), strings.TrimRight(stderr.String(), "\n"))
	}
	result["config"] = stdout.String()

	return result, nil
}
