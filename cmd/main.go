package main

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.

import (
	"errors"
	"fmt"
	"github.com/appliedtrust/sweet"
	"github.com/docopt/docopt-go"
	"github.com/vaughan0/go-ini"
	"log/syslog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const version = "1.4-dev"

var usage = `sweet: network device backups and change alerts for the 21st century.

Usage:
  sweet [options] <config>
  sweet -h --help
  sweet --version

Options:
  -w, --workspace <dir>     Specify workspace directory (default: ./sweet-workspace).
  -i, --interval <secs>     Collection interval in secs (default: 300).
  -c, --concurrency <num>   Concurrent device collections (default: 30).
  -t, --to <email@addr>     Send change notifications to this email.
  -f, --from <email@addr>   Send change notifications from this email.
  -s, --smtp <host:port>    SMTP server connection info (default: localhost:25).
  --insecure                Accept untrusted SSH device keys.
  --push                    Do a "git push" after committing changed configs.
  --syslog                  Send log messages to syslog rather than stdout.
  --timeout <secs>          Device collection timeout in secs (default: 60).
  --web                     Run an HTTP status server.
  --weblisten <host:port>   Host and port to use for HTTP status server (default: localhost:5000).
  --webauth <user:pass>     Optional user/pass to protect HTTP status server.
  --version                 Show version.
  -h, --help                Show this screen.
`

//// here we go...
func main() {
	Opts, err := setupOptions()
	if err != nil {
		Opts.LogFatal(err.Error())
	}
	go sweet.RunWebserver(&Opts)
	sweet.RunCollectors(&Opts)
}

//// Read CLI flags and config file
func setupOptions() (sweet.SweetOptions, error) {
	Opts := sweet.SweetOptions{}
	Opts.Status = &sweet.Status{}
	Opts.Status.Status = make(map[string]sweet.DeviceStatus)

	arguments, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		return Opts, err
	}

	// check for runtime dependencies
	if _, err := exec.LookPath("git"); err != nil {
		return Opts, errors.New("Couldn't find the git command in your path.")
	}

	Opts.ExecutableDir, err = os.Getwd()
	if err != nil {
		return Opts, err
	}

	// set non-zero-value defaults
	Opts.Workspace = "./sweet-workspace"
	Opts.Concurrency = 30
	Opts.SmtpString = "localhost:25"
	Opts.HttpListen = "localhost:5000"
	Opts.Interval = 300 * time.Second
	Opts.Timeout = 60 * time.Second
	Opts.Insecure = false
	Opts.GitPush = false
	Opts.UseSyslog = false
	Opts.HttpEnabled = false

	// read in config file - config file options override defaults if set
	configFile, err := ini.LoadFile(arguments["<config>"].(string))
	if err != nil {
		return Opts, err
	}

	section, ok := configFile[""]
	if !ok {
		return Opts, err
	}
	if _, ok := section["workspace"]; ok {
		Opts.Workspace = section["workspace"]
	}
	if _, ok = section["to"]; ok {
		Opts.ToEmail = section["to"]
	}
	if _, ok = section["from"]; ok {
		Opts.FromEmail = section["from"]
	}
	if _, ok = section["smtp"]; ok {
		Opts.SmtpString = section["smtp"]
	}
	if _, ok = section["weblisten"]; ok {
		Opts.HttpListen = section["weblisten"]
	}
	if _, ok = section["webauth"]; ok {
		parts := strings.SplitN(":", section["webauth"], 2)
		if len(parts) != 2 {
			return Opts, fmt.Errorf("Malformed webauth argument - should be username:pass")
		}
		Opts.HttpUser = parts[0]
		Opts.HttpPass = parts[1]
	}
	if _, ok = section["concurrency"]; ok {
		Opts.Concurrency, err = strconv.Atoi(section["concurrency"])
		if err != nil {
			return Opts, err
		}
	}

	if boolText, ok := section["insecure"]; ok {
		if boolText == "true" {
			Opts.Insecure = true
		}
	}
	if boolText, ok := section["push"]; ok {
		if boolText == "true" {
			Opts.GitPush = true
		}
	}
	if boolText, ok := section["syslog"]; ok {
		if boolText == "true" {
			Opts.UseSyslog = true
		}
	}
	if boolText, ok := section["web"]; ok {
		if boolText == "true" {
			Opts.HttpEnabled = true
		}
	}
	if intervalText, ok := section["interval"]; ok {
		Opts.Interval, err = time.ParseDuration(intervalText + "s")
		if err != nil {
			return Opts, err
		}
	}
	if timeoutText, ok := section["timeout"]; ok {
		Opts.Timeout, err = time.ParseDuration(timeoutText + "s")
		if err != nil {
			return Opts, err
		}
	}
	if d, ok := section["default-user"]; ok {
		Opts.DefaultUser = d
	}
	if d, ok := section["default-pass"]; ok {
		Opts.DefaultPass = d
	}
	if d, ok := section["default-method"]; ok {
		Opts.DefaultMethod = d
	}
	for name, section := range configFile {
		if len(name) == 0 { // did global config first above
			continue
		} else { // device-specific config
			device := sweet.DeviceConfig{Hostname: name, Method: section["method"], Config: section}
			if len(device.Method) == 0 {
				if len(Opts.DefaultMethod) == 0 {
					return Opts, fmt.Errorf("No method specified for %s and default-method not defined.", device.Hostname)
				}
				device.Method = Opts.DefaultMethod
			}

			// timeouts
			if _, ok := device.Config["timeout"]; !ok {
				device.Timeout = Opts.Timeout
			} else {
				device.Timeout, err = time.ParseDuration(device.Config["timeout"] + "s")
				if err != nil {
					return Opts, fmt.Errorf("Bad timeout setting %s for host %s", device.Config["timeout"], device.Hostname)
				}
			}
			if _, ok := device.Config["user"]; !ok {
				if len(Opts.DefaultUser) == 0 {
					return Opts, fmt.Errorf("No user specified for %s and default-user not defined.", device.Hostname)
				}
				device.Config["user"] = Opts.DefaultUser
			}
			if _, ok := device.Config["pass"]; !ok {
				if len(Opts.DefaultPass) == 0 {
					return Opts, fmt.Errorf("No pass specified for %s and default-pass not defined.", device.Hostname)
				}
				device.Config["pass"] = Opts.DefaultPass
			}
			// use normal pw for enable if enable not specified
			if _, ok := device.Config["enable"]; !ok {
				device.Config["enable"] = device.Config["pass"]
			}
			device.Target = device.Hostname
			if _, ok := device.Config["ip"]; ok {
				device.Target = device.Config["ip"]
			}
			if Opts.Insecure {
				device.Config["insecure"] = "true"
			}
			Opts.Devices = append(Opts.Devices, device)
		}
	}

	// CLI flags override config file opts if set
	if arguments["--syslog"].(bool) {
		Opts.UseSyslog = arguments["--syslog"].(bool)
		// setup logging now
		Opts.Syslog, err = syslog.New(syslog.LOG_ALERT|syslog.LOG_USER, "sweet")
		if err != nil {
			return Opts, err
		}
	}
	if arguments["--web"].(bool) {
		Opts.HttpEnabled = true
	}
	if arguments["--push"].(bool) {
		Opts.GitPush = true
	}
	if arguments["--insecure"].(bool) {
		Opts.Insecure = true
	}
	if arguments["--smtp"] != nil {
		Opts.SmtpString = arguments["--smtp"].(string)
	}
	if arguments["--to"] != nil {
		if arguments["--from"] == nil {
			return Opts, errors.New("Both --to and --from arguments required for email to work.")
		}
		Opts.ToEmail = arguments["--to"].(string)
	}
	if arguments["--from"] != nil {
		if arguments["--to"] == nil {
			return Opts, errors.New("Both --to and --from arguments required for email to work.")
		}
		Opts.FromEmail = arguments["--from"].(string)
	}
	if arguments["--weblisten"] != nil {
		Opts.HttpListen = arguments["--weblisten"].(string)
	}
	if arguments["--webauth"] != nil {
		parts := strings.SplitN(arguments["--webauth"].(string), ":", 2)
		if len(parts) != 2 {
			return Opts, fmt.Errorf("Malformed webauth argument - should be username:pass ")
		}
		Opts.HttpUser = parts[0]
		Opts.HttpPass = parts[1]
	}

	// concurrent collectors
	if arguments["--concurrency"] != nil {
		Opts.Concurrency, err = strconv.Atoi(arguments["--concurrency"].(string))
		if err != nil {
			return Opts, err
		}
	}

	// collection interval and timeouts
	if arguments["--interval"] != nil {
		Opts.Interval, err = time.ParseDuration(arguments["--interval"].(string) + "s")
		if err != nil {
			return Opts, err
		}
	}
	if arguments["--timeout"] != nil {
		Opts.Timeout, err = time.ParseDuration(arguments["--timeout"].(string) + "s")
		if err != nil {
			return Opts, err
		}
	}

	// configs get saved in workspace directory
	if arguments["--workspace"] != nil {
		Opts.Workspace = arguments["--workspace"].(string)
	}

	if _, err := os.Stat(Opts.Workspace); err != nil {
		if err := os.MkdirAll(Opts.Workspace, 0755); err != nil {
			return Opts, err
		}
	}

	// switch to workspace directory
	err = os.Chdir(Opts.Workspace)
	if err != nil {
		return Opts, err
	}

	// make sure git is ready to go
	if _, err := os.Stat(".git"); err != nil {
		if _, err := exec.Command("git", "init").Output(); err != nil {
			return Opts, err
		}
	}

	return Opts, nil
}
