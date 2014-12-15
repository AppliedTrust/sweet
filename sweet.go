package sweet

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.

import (
	"fmt"
	"github.com/kr/pty"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	recentHours = 12
)

// DeviceAccess stores host access info
type DeviceConfig struct {
	Hostname       string
	Method         string
	Target         string
	Timeout        time.Duration
	CommandTimeout time.Duration
	Config         map[string]string
}

type DeviceStatusState int

const (
	StatePending DeviceStatusState = iota
	StateError
	StateTimeout
	StateSuccess
)

type ConfigDiff struct {
	Diff    string
	Added   int
	Removed int
	NewFile bool
}

type DeviceStatus struct {
	Device       DeviceConfig
	State        DeviceStatusState
	When         time.Time
	Configs      map[string]string
	Diffs        map[string]ConfigDiff
	ErrorMessage string
}
type Status struct {
	Status map[string]DeviceStatus
	Lock   sync.Mutex
}

// ReportWebData options for formatting web status page.
type WebReport struct {
	DeviceStatus
	Class          string
	CSSID          string
	EnableDiffLink bool
	EnableConfLink bool
}

type SSHCollector struct {
	Receive chan string
	Send    chan string
}

type SweetOptions struct {
	Interval      time.Duration
	Timeout       time.Duration
	GitPush       bool
	Insecure      bool
	Concurrency   int
	HttpListen    string
	HttpEnabled   bool
	SmtpString    string
	Workspace     string
	ExecutableDir string
	ToEmail       string
	FromEmail     string
	UseSyslog     bool
	DefaultUser   string
	DefaultPass   string
	DefaultMethod string
	Syslog        *syslog.Writer
	Devices       []DeviceConfig
	Status        *Status
}

type Collector interface {
	Collect(device DeviceConfig) (map[string]string, error)
}

//// Kickoff collector runs
func RunCollectors(Opts *SweetOptions) {
	collectorSlots := make(chan bool, Opts.Concurrency)
	for {
		end := time.Now().Add(Opts.Interval)
		Opts.LogInfo(fmt.Sprintf("Starting %d collectors. [concurrency=%d]", len(Opts.Devices), Opts.Concurrency))

		go func() {
			for _, device := range Opts.Devices {
				collectorSlots <- true
				status := DeviceStatus{}
				status.Device = device
				status.When = time.Now()
				status.State = StatePending
				Opts.Status.Set(status)

				Opts.LogInfo(fmt.Sprintf("Starting collector: %s", device.Hostname))
				status = collectDevice(device, Opts)
				Opts.LogInfo(fmt.Sprintf("Finished collector: %s", device.Hostname))
				Opts.Status.Set(status)
			}
			Opts.LogInfo(fmt.Sprintf("All %d collectors finished.", len(Opts.Devices)))
			if err := updateDiffs(Opts); err != nil {
				Opts.LogFatal(err.Error())
			}
			if err := commitChanges(Opts); err != nil {
				Opts.LogFatal(err.Error())
			}
			if err := runReporter(Opts); err != nil {
				Opts.LogFatal(err.Error())
			}
			if Opts.Interval == 0 {
				Opts.LogInfo("Interval set to 0 - exiting.")
				os.Exit(0)
			}
		}()
		Opts.LogInfo(fmt.Sprintf("Waiting for %d collectors.", len(Opts.Devices)))
		for i := 0; i < len(Opts.Devices); i++ {
			_ = <-collectorSlots
			//Opts.LogInfo(fmt.Sprintf("Collector returned."))
		}
		Opts.LogInfo(fmt.Sprintf("Started all %d collectors.", len(Opts.Devices)))
		if end.After(time.Now()) {
			time.Sleep(end.Sub(time.Now()))
		}
	}
}

//// Get and save config from a single device
func collectDevice(device DeviceConfig, Opts *SweetOptions) DeviceStatus {
	var err error

	status := DeviceStatus{}
	status.Device = device
	status.When = time.Now()

	if len(device.Method) == 0 {
		if len(Opts.DefaultMethod) == 0 {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("No method specified for %s and default-method not defined.", device.Hostname)
			return status
		}
		device.Method = Opts.DefaultMethod
	}

	// override timeouts in device configs
	device.Timeout = Opts.Timeout
	_, ok := device.Config["timeout"]
	if ok {
		device.Timeout, err = time.ParseDuration(device.Config["timeout"] + "s")
		if err != nil {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("Bad timeout setting %s for host %s", device.Config["timeout"], device.Hostname)
			return status
		}
	}
	device.CommandTimeout = Opts.Timeout
	_, ok = device.Config["commandtimeout"]
	if ok {
		device.CommandTimeout, err = time.ParseDuration(device.Config["commandtimeout"] + "s")
		if err != nil {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("Bad command timeout setting %s for host %s", device.Config["commandtimeout"], device.Hostname)
			return status
		}
	}
	// setup collection options
	_, ok = device.Config["user"]
	if !ok {
		if len(Opts.DefaultUser) == 0 {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("No user specified for %s and default-user not defined.", device.Hostname)
			return status
		}
		device.Config["user"] = Opts.DefaultUser
	}
	_, ok = device.Config["pass"]
	if !ok {
		if len(Opts.DefaultPass) == 0 {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("No pass specified for %s and default-pass not defined.", device.Hostname)
			return status
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

	var c Collector
	if device.Method == "cisco" {
		c = newCiscoCollector()
	} else if device.Method == "external" {
		// handle absolute and relative script paths
		device.Config["scriptPath"] = device.Config["script"]
		if device.Config["script"][0] != os.PathSeparator {
			device.Config["scriptPath"] = Opts.ExecutableDir + string(os.PathSeparator) + device.Config["script"]
		}
		c = newExternalCollector()
	} else if device.Method == "junos" {
		c = newJunOSCollector()
	} else {
		status.State = StateError
		status.ErrorMessage = fmt.Sprintf("Unknown access method: %s", device.Method)
		return status
	}

	var collectionResults map[string]string
	r := make(chan map[string]string)
	e := make(chan error)
	go func() {
		result, err := c.Collect(device)
		if err != nil {
			e <- err
		} else {
			r <- result
		}
	}()
	select {
	case collectionResults = <-r:
	case <-time.After(Opts.Timeout):
		status.State = StateError
		status.ErrorMessage = fmt.Sprintf("collection timeout after %d seconds", int(device.Timeout.Seconds()))
		return status
	case err := <-e:
		status.State = StateError
		status.ErrorMessage = fmt.Sprintf("collection error: %s", err.Error())
		return status
	}

	// save the collectionResults to the workspace
	for name, val := range collectionResults {
		Opts.LogInfo(fmt.Sprintf("Saving result: %s %s", device.Hostname, name))
		err = ioutil.WriteFile(device.Hostname+"-"+cleanName(name), []byte(val), 0644)
		if err != nil {
			status.State = StateError
			status.ErrorMessage = fmt.Sprintf("Error saving %s result to workspace: %s", name, err.Error())
			return status
		}
	}
	status.State = StateSuccess
	status.Configs = collectionResults
	return status
}

func newSSHCollector(device DeviceConfig) (*SSHCollector, error) {
	c := new(SSHCollector)
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

func (s *Status) Get(device string) DeviceStatus {
	defer func() {
		s.Lock.Unlock()
	}()
	s.Lock.Lock()
	return s.Status[device]
}
func (s *Status) GetAll() map[string]DeviceStatus {
	defer func() {
		s.Lock.Unlock()
	}()
	s.Lock.Lock()
	return s.Status
}

func (s *Status) Set(stat DeviceStatus) {
	defer func() {
		s.Lock.Unlock()
	}()
	s.Lock.Lock()
	s.Status[stat.Device.Hostname] = stat
}
