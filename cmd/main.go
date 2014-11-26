package main

// sweet.go: network device backups and change alerts for the 21st century - inspired by RANCID.

// TODO: send email on start, new errors

import (
	"errors"
	"github.com/appliedtrust/sweet"
	"github.com/docopt/docopt-go"
	"github.com/vaughan0/go-ini"
	"log/syslog"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const version = "1.2"

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
  --version                 Show version.
  -h, --help                Show this screen.
`

//// here we go...
func main() {
	Opts, err := setupOptions()
	if err != nil {
		Opts.LogFatal(err.Error())
	}

	ec, err := sweet.RunErrorCache()
	if err != nil {
		Opts.LogFatal(err.Error())
	}
	Opts.ErrorCacheUpdates = ec.Updates
	Opts.ErrorCacheRequests = ec.Requests

	go sweet.RunWebserver(&Opts)

	sweet.RunCollectors(&Opts)
}

//// Read CLI flags and config file
func setupOptions() (sweet.SweetOptions, error) {
	Opts := sweet.SweetOptions{}
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
	Opts.SmtpString = "localhost:25" //TODO BROKEN!?
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

	for name, section := range configFile {
		if len(name) == 0 { // global config
			_, ok := section["workspace"]
			if ok {
				Opts.Workspace = section["workspace"]
			}
			_, ok = section["to"]
			if ok {
				Opts.ToEmail = section["to"]
			}
			_, ok = section["from"]
			if ok {
				Opts.FromEmail = section["from"]
			}
			_, ok = section["smtp"]
			if ok {
				Opts.SmtpString = section["smtp"]
			}
			_, ok = section["weblisten"]
			if ok {
				Opts.HttpListen = section["weblisten"]
			}
			_, ok = section["concurrency"]
			if ok {
				Opts.Concurrency, err = strconv.Atoi(section["concurrency"])
				if err != nil {
					return Opts, err
				}
			}

			boolText, ok := section["insecure"]
			if ok {
				if boolText == "true" {
					Opts.Insecure = true
				}
			}
			boolText, ok = section["push"]
			if ok {
				if boolText == "true" {
					Opts.GitPush = true
				}
			}
			boolText, ok = section["syslog"]
			if ok {
				if boolText == "true" {
					Opts.UseSyslog = true
				}
			}
			boolText, ok = section["web"]
			if ok {
				if boolText == "true" {
					Opts.HttpEnabled = true
				}
			}
			intervalText, ok := section["interval"]
			if ok {
				Opts.Interval, err = time.ParseDuration(intervalText + "s")
				if err != nil {
					return Opts, err
				}
			}
			timeoutText, ok := section["timeout"]
			if ok {
				Opts.Timeout, err = time.ParseDuration(timeoutText + "s")
				if err != nil {
					return Opts, err
				}
			}

			defaultUser, ok := section["default-user"]
			if ok {
				Opts.DefaultUser = defaultUser
			}
			defaultPass, ok := section["default-pass"]
			if ok {
				Opts.DefaultPass = defaultPass
			}
			defaultMethod, ok := section["default-method"]
			if ok {
				Opts.DefaultMethod = defaultMethod
			}

		} else { // device-specific config
			device := new(sweet.DeviceAccess)
			device.Hostname = name
			device.Method = section["method"]
			device.Config = section
			Opts.Devices = append(Opts.Devices, *device)
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

//// EOF
