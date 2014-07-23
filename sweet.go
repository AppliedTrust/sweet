package sweet

// sweet.go: network device backups and change alerts for the 21st century - it's not RANCID.

// TODO: send email on start, new errors

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/kballard/go-shellquote"
	"github.com/kr/pty"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	recentHours = 12
)

// DeviceAccess stores host info from the configuration file.
type DeviceAccess struct {
	Hostname string
	Method   string
	Target   string
	Timeout  time.Duration
	Config   map[string]string
}

// Report is based on Git data for web or email interface.
type Report struct {
	Device        DeviceAccess
	Error         error
	Diff          string
	StatusMessage string
	Added         string
	Removed       string
	CollectedTime time.Time
	ChangedTime   time.Time
	Web           ReportWebData
}

// ReportWebData options for formatting web status page.
type ReportWebData struct {
	Class          string
	CSSID          string
	EnableDiffLink bool
	EnableConfLink bool
}

type Collector struct {
	Receive chan string
	Send    chan string
}

type SweetOptions struct {
	Interval           time.Duration
	Timeout            time.Duration
	GitPush            bool
	Insecure           bool
	Concurrency        int
	HttpListen         string
	HttpEnabled        bool
	SmtpString         string
	Workspace          string
	ExecutableDir      string
	ToEmail            string
	FromEmail          string
	UseSyslog          bool
	DefaultUser        string
	DefaultPass        string
	DefaultMethod      string
	ErrorCacheUpdates  chan *ErrorCacheUpdate
	ErrorCacheRequests chan *ErrorCacheRequest
	Syslog             *syslog.Writer
	Devices            []DeviceAccess
}

//// Kickoff collector runs
func RunCollectors(Opts *SweetOptions) {
	collectorSlots := make(chan bool, Opts.Concurrency)

	for {
		Opts.LogInfo(fmt.Sprintf("Starting %d collectors. [concurrency=%d]", len(Opts.Devices), Opts.Concurrency))
		done := make(chan string, len(Opts.Devices))

		go func() {
			for _, device := range Opts.Devices {
				collectorSlots <- true
				go collectDevice(device, Opts, done)
				//opts.logInfo(fmt.Sprintf("Collector started:  %s", device.Hostname))
			}
		}()
		Opts.LogInfo(fmt.Sprintf("Waiting for %d collectors.", len(Opts.Devices)))
		for i := 0; i < len(Opts.Devices); i++ {
			_ = <-collectorSlots
			doneHostname := <-done
			_ = doneHostname
			//opts.logInfo(fmt.Sprintf("Collector returned: %s", doneHostname))
		}
		Opts.LogInfo(fmt.Sprintf("Finished with all %d collectors.", len(Opts.Devices)))

		statusText, err := exec.Command("git", "status", "-s").Output()
		if err != nil {
			Opts.LogFatal(err)
		}
		if len(statusText) > 0 {
			_, err = exec.Command("git", "add", ".").Output()
			if err != nil {
				Opts.LogFatal(err)
			}

			commitMsg := "Sweet commit:\n" + string(statusText)
			_, err = exec.Command("git", "commit", "-a", "-m", commitMsg).Output()
			if err != nil {
				Opts.LogFatal(err)
			}
			if Opts.GitPush == true {
				_, err = exec.Command("git", "push").Output()
				if err != nil {
					Opts.LogErr(fmt.Sprintf("Git push failed, continuing anyway: %s", err.Error()))
				}
			}

			go runReporter(*Opts)
			Opts.LogInfo(fmt.Sprintf("Committed changes to git."))
		} else {
			Opts.LogInfo(fmt.Sprintf("No changes detected."))
		}

		if Opts.Interval == 0 {
			Opts.LogInfo("Interval set to 0 - exiting.")
			os.Exit(0)
		}
		time.Sleep(Opts.Interval)
	}
}

//// Get and save config from a single device
func collectDevice(device DeviceAccess, Opts *SweetOptions, done chan string) {
	var err error

	if len(device.Method) == 0 {
		if len(Opts.DefaultMethod) == 0 {
			Opts.LogFatal(fmt.Errorf("No method specified for %s and default-method not defined.", device.Hostname))
		}
		device.Method = Opts.DefaultMethod
	}

	// override timeouts in device configs
	device.Timeout = Opts.Timeout
	_, ok := device.Config["timeout"]
	if ok {
		device.Timeout, err = time.ParseDuration(device.Config["timeout"] + "s")
		if err != nil {
			Opts.LogFatal(fmt.Errorf("Bad timeout setting %s for host %s", device.Config["timeout"], device.Hostname))
		}
	}
	// setup collection options
	_, ok = device.Config["user"]
	if !ok {
		if len(Opts.DefaultUser) == 0 {
			Opts.LogFatal(fmt.Errorf("No user specified for %s and default-user not defined.", device.Hostname))
		}
		device.Config["user"] = Opts.DefaultUser
	}
	_, ok = device.Config["pass"]
	if !ok {
		if len(Opts.DefaultPass) == 0 {
			Opts.LogFatal(fmt.Errorf("No pass specified for %s and default-pass not defined.", device.Hostname))
		}
		device.Config["pass"] = Opts.DefaultPass
	}
	_, ok = device.Config["enable"]
	if !ok {
		device.Config["enable"] = device.Config["pass"]
	}
	device.Target = device.Hostname
	_, ok = device.Config["ip"]
	if ok {
		device.Target = device.Config["ip"]
	}
	if Opts.Insecure {
		device.Config["insecure"] = "true"
	}

	rawConfig := ""

	if device.Method == "external" {
		// handle absolute and relative script paths
		device.Config["scriptPath"] = device.Config["script"]
		if device.Config["script"][0] != os.PathSeparator {
			device.Config["scriptPath"] = Opts.ExecutableDir + string(os.PathSeparator) + device.Config["script"]
		}

		rawConfig, err = collectExternal(device)
		if err != nil {
			Opts.LogErr(err.Error())
			Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: err.Error()}
			done <- device.Hostname
			return
		}

	} else if device.Method == "cisco" {
		r := make(chan map[string]string)
		go func() {
			r <- CollectCisco(device)
		}()
		collectionResults := make(map[string]string)
		select {
		case collectionResults = <-r:
		case <-time.After(Opts.Timeout):
			msg := fmt.Sprintf("Timeout collecting from %s after %d seconds", device.Hostname, int(device.Timeout.Seconds()))
			Opts.LogErr(msg)
			Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: msg}
			done <- device.Hostname
			return
		}
		if len(collectionResults["err"]) > 0 {
			Opts.LogErr(collectionResults["err"])
			Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: collectionResults["err"]}
			done <- device.Hostname
			return
		}
		// for now we only handle config collectionResults
		rawConfig, ok = collectionResults["config"]
		if !ok {
			msg := fmt.Sprintf("Config missing from collection results", device.Hostname)
			Opts.LogErr(msg)
			Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: msg}
			done <- device.Hostname
			return
		}

	} else {
		msg := fmt.Sprintf("Unknown access method: %s", device.Method)
		Opts.LogErr(msg)
		Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: msg}
		done <- device.Hostname
		return
	}

	// save the config to the workspace
	err = ioutil.WriteFile(device.Hostname, []byte(rawConfig), 0644)
	if err != nil {
		Opts.LogFatal(err)
	}

	// notify runCollectors() that we're done
	Opts.ErrorCacheUpdates <- &ErrorCacheUpdate{Hostname: device.Hostname, ErrorMessage: ""}
	done <- device.Hostname
}

func collectExternal(device DeviceAccess) (string, error) {
	var cmd *exec.Cmd

	commandParts, err := shellquote.Split(device.Config["scriptPath"])
	if err != nil {
		return "", nil
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
		return "", nil
	}

	cmdDone := make(chan error)

	go func() {
		cmdDone <- cmd.Wait()
	}()

	select {
	case err := <-cmdDone:
		if err != nil {
			errMessage := strings.TrimRight(stderr.String(), "\n") + " " + err.Error()
			return "", fmt.Errorf("Error collecting from %s: %s", device.Hostname, errMessage)
		}
	case <-time.After(device.Timeout):
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return "", err
		}
		return "", errors.New(fmt.Sprintf("Timeout collecting from %s after %d seconds", device.Hostname, int(device.Timeout.Seconds())))
	}
	// TODO: cleanup external script output to not include SSH session junk
	return stdout.String(), nil
}

func newSSHCollector(device DeviceAccess) (*Collector, error) {
	c := new(Collector)
	c.Receive = make(chan string)
	c.Send = make(chan string)

	var cmd *exec.Cmd
	_, ok := device.Config["insecure"]
	if ok && device.Config["insecure"] == "true" {
		cmd = exec.Command("ssh", "-oStrictHostKeyChecking=no", device.Config["user"]+"@"+device.Target)
	} else {
		cmd = exec.Command("ssh", device.Config["user"]+"@"+device.Target)
	}

	f, err := pty.Start(cmd)
	if err != nil {
		return c, err
	}

	go func() {
		for {
			str, err := readChunk(f)
			if err != nil {
				close(c.Receive)
				return
			}
			c.Receive <- str
		}
	}()

	go func() {
		for {
			select {
			case command, exists := <-c.Send:
				{
					if !exists {
						return
					}
					_, err := io.WriteString(f, command)
					if err != nil {
						panic("send error")
					}
				}
			}
		}
	}()

	return c, nil
}
