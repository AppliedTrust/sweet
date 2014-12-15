package sweet

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.

import (
	"bytes"
	"fmt"
	"github.com/kballard/go-shellquote"
	"os"
	"os/exec"
	"strings"
	"time"
)

type External struct {
}

func newExternalCollector() Collector {
	return External{}
}

func (collector External) Collect(device DeviceConfig) (map[string]string, error) {
	var cmd *exec.Cmd
	result := make(map[string]string)

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

	cmdDone := make(chan error)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	select {
	case err := <-cmdDone:
		if err != nil {
			return result, fmt.Errorf("External collection script (%s) returned an error: %s - %s", device.Config["scriptPath"], err.Error(), strings.TrimRight(stderr.String(), "\n"))
		}
	case <-time.After(device.Timeout):
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return result, fmt.Errorf("Error stopping external collection script (%s) after timeout: %s", device.Config["scriptPath"], err.Error())
		}
		return result, fmt.Errorf("Timeout collecting from %s after %d seconds", device.Hostname, int(device.Timeout.Seconds()))
	}
	result["config"] = stdout.String()

	return result, nil
}
