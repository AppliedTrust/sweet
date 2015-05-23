package sweet

//TODO: tests, yo

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.
import (
	"bufio"
	"fmt"
	"github.com/kr/pty"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type DeviceStatusState int

const recentHours = 12

const (
	StatePending DeviceStatusState = iota
	StateError
	StateTimeout
	StateSuccess
)

// DeviceConfig stores a device's access info
type DeviceConfig struct {
	Hostname string
	Method   string
	Target   string
	Timeout  time.Duration
	Config   map[string]string `json:"-"`
}

// ConfigDiff stores changes in individual device configs
type ConfigDiff struct {
	Diff    string
	Added   int
	Removed int
	NewFile bool
}

// DeviceStatus stores the status of a single device
type DeviceStatus struct {
	Device        DeviceConfig
	State         DeviceStatusState
	StatePrevious DeviceStatusState
	When          time.Time
	Configs       map[string]string
	Diffs         map[string]ConfigDiff
	ErrorMessage  string
}

// Status provides a global, lockable DeviceStatus for all devices
type Status struct {
	Status map[string]DeviceStatus
	Lock   sync.Mutex
}

// Connection handles a single SSH or shell session with a device
type Connection struct {
	Receive chan string
	Send    chan string
	Cmd     *exec.Cmd
}

// SweetOptions stores user-configurable settings
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
	DefaultPass   string `json:"-"`
	DefaultMethod string
	HttpUser      string `json:"-"`
	HttpPass      string `json:"-"`
	Devices       []DeviceConfig
	Syslog        *syslog.Writer `json:"-"`
	Status        *Status        `json:"-"`
	Hub           *Hub           `json:"-"`
}

// CollectionResults contains device configs for a single device
type CollectionResults map[string]string

// a Collector can use a device and SSH connection to return collection results
type Collector interface {
	Collect(device DeviceConfig, conn *Connection) (CollectionResults, error)
}

// RunCollectors kicks off collector runs for all devices on a schedule
func RunCollectors(Opts *SweetOptions) {
	for {
		end := time.Now().Add(Opts.Interval)
		Opts.LogInfo(fmt.Sprintf("Starting %d collectors. [concurrency=%d]", len(Opts.Devices), Opts.Concurrency))
		collectorSlots := make(chan bool, Opts.Concurrency)
		collectorDones := make(chan bool)
		go func() {
			for _, device := range Opts.Devices {
				collectorSlots <- true
				device := device
				go func() {
					Opts.LogInfo(fmt.Sprintf("Starting collector: %s", device.Hostname))
					status := DeviceStatus{}
					status.Device = device
					status.When = time.Now()
					status.StatePrevious = status.State
					status.State = StatePending
					Opts.StatusSet(status)

					status = collectDevice(device, Opts)
					Opts.StatusSet(status)
					Opts.LogInfo(fmt.Sprintf("Finished collector: %s", device.Hostname))
					_ = <-collectorSlots
					collectorDones <- true
				}()
			}
			Opts.LogInfo(fmt.Sprintf("Started all %d collectors.", len(Opts.Devices)))
		}()
		for i := 0; i < len(Opts.Devices); i++ {
			_ = <-collectorDones
		}
		Opts.LogInfo(fmt.Sprintf("All %d collectors finished.", len(Opts.Devices)))
		if err := updateDiffs(Opts); err != nil {
			Opts.LogFatal(fmt.Sprintf("Fatal error updating config diffs: %s", err.Error()))
		}
		if err := commitChanges(Opts); err != nil {
			Opts.LogFatal(fmt.Sprintf("Fatal error commiting changes with git: %s", err.Error()))
		}
		if err := runReporter(Opts); err != nil {
			Opts.LogFatal(fmt.Sprintf("Fatal error sending email report: %s", err.Error()))
		}
		if Opts.Interval == 0 {
			Opts.LogInfo("Interval set to 0 - exiting.")
			os.Exit(0)
		}
		if end.After(time.Now()) {
			Opts.LogInfo(fmt.Sprintf("Sleeping %d seconds.", int(end.Sub(time.Now()).Seconds())))
			time.Sleep(end.Sub(time.Now()))
		}
	}
}

// collectDevice gets and saves config from a single device
func collectDevice(device DeviceConfig, Opts *SweetOptions) DeviceStatus {
	status := DeviceStatus{}
	status.Device = device
	status.When = time.Now()
	status.StatePrevious = status.State
	status.State = StateError

	var c Collector
	if device.Method == "cisco" {
		c = newCiscoCollector()
	} else if device.Method == "junos" {
		c = newJunOSCollector()
	} else if device.Method == "external" {
		// handle absolute and relative script paths
		device.Config["scriptPath"] = device.Config["script"]
		if device.Config["script"][0] != os.PathSeparator {
			device.Config["scriptPath"] = Opts.ExecutableDir + string(os.PathSeparator) + device.Config["script"]
		}
		c = newExternalCollector()
	} else {
		status.StatePrevious = status.State
		status.State = StateError
		status.ErrorMessage = fmt.Sprintf("Unknown access method for %s: %s", device.Hostname, device.Method)
		return status
	}
	var collectionResults map[string]string
	r := make(chan map[string]string)
	e := make(chan error)
	session, err := newConnection(device) // TODO: external ExternalCollector doesn't need a SSH connection!
	if err != nil {
		status.ErrorMessage = fmt.Sprintf("Unable to connect to %s at %s", device.Hostname, device.Target)
		Opts.LogErr(status.ErrorMessage)
		return status
	}
	defer func() {
		close(session.Send)
		close(session.Receive)
		close(r)
		close(e)
		if err := session.Cmd.Process.Kill(); err != nil {
			log.Printf("Error killing ssh process")
		}
		if err := session.Cmd.Wait(); err != nil {
			//log.Printf("Error waiting for ssh exit")
		}
	}()
	go func() {
		result, err := c.Collect(device, session)
		if err != nil {
			e <- err
		} else {
			r <- result
		}
	}()
	select {
	case collectionResults = <-r:
	case err := <-e:
		status.ErrorMessage = fmt.Sprintf("Collection error for %s: %s", device.Hostname, err.Error())
		Opts.LogErr(status.ErrorMessage)
		return status
	}

	// save the collectionResults to the workspace
	for name, val := range collectionResults {
		Opts.LogInfo(fmt.Sprintf("Saving result: %s %s", device.Hostname, name))
		if err := ioutil.WriteFile(device.Hostname+"-"+cleanName(name), []byte(val), 0644); err != nil {
			status.ErrorMessage = fmt.Sprintf("Error saving %s result to workspace: %s", name, err.Error())
			Opts.LogErr(status.ErrorMessage)
			return status
		}
	}
	status.StatePrevious = status.State
	status.State = StateSuccess
	status.Configs = collectionResults
	return status
}

// newConnection establishes a connection to a device
func newConnection(device DeviceConfig) (*Connection, error) {
	c := Connection{}
	c.Receive = make(chan string)
	c.Send = make(chan string)
	if _, ok := device.Config["insecure"]; ok && device.Config["insecure"] == "true" {
		c.Cmd = exec.Command("ssh", "-oStrictHostKeyChecking=no", device.Config["user"]+"@"+device.Target)
	} else {
		c.Cmd = exec.Command("ssh", device.Config["user"]+"@"+device.Target)
	}

	f, err := pty.Start(c.Cmd)
	if err != nil {
		return &c, err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Error (panic) reading from SSH connection: %s", r)
			}
			f.Close()
		}()
		reader := bufio.NewReader(f)
		for {
			buf := make([]byte, 1024)
			if _, err := reader.Read(buf); err != nil {
				if err != io.EOF {
					//log.Printf("Error reading from SSH connection: %s", err.Error())
				}
				return
			}
			c.Receive <- string(buf)
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Error (panic) writing to SSH connection: %s", r)
			}
			f.Close()
		}()
		for {
			select {
			case command, exists := <-c.Send:
				{
					if !exists {
						return
					}
					_, err := f.Write([]byte(command))
					if err != nil {
						if err != io.EOF {
							log.Printf("Error writing to SSH connection: %s", err.Error())
						}
						return
					}
				}
			}
		}
	}()
	return &c, nil
}

// StatusGet safely gets a device's status from global state
func (Opts *SweetOptions) StatusGet(device string) DeviceStatus {
	defer func() {
		Opts.Status.Lock.Unlock()
	}()
	Opts.Status.Lock.Lock()
	return Opts.Status.Status[device]
}

// StatusGetAll safely gets all devices' status from global state
func (Opts *SweetOptions) StatusGetAll() map[string]DeviceStatus {
	defer func() {
		Opts.Status.Lock.Unlock()
	}()
	Opts.Status.Lock.Lock()
	return Opts.Status.Status
}

// StatusSet safely sets a device's status in global state
func (Opts *SweetOptions) StatusSet(stat DeviceStatus) {
	if Opts.Hub != nil {
		Opts.Hub.broadcast <- event{MessageType: "device", Device: deviceId(stat.Device.Hostname), Status: stat}
	}
	defer func() {
		Opts.Status.Lock.Unlock()
	}()
	Opts.Status.Lock.Lock()
	Opts.Status.Status[stat.Device.Hostname] = stat
}

// deviceId provides a clean device id for use in the frontend
func deviceId(d string) string {
	return strings.Replace(d, ".", "", -1)
}
