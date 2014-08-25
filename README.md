Network device configuration backups and change alerts for the 21st century - inspired by RANCID!

##Features:
* Stores device configs in Git
* Simple configuration file
* Single binary - only runtime dependency is Git
* Email notifications
* Built-in web status dashboard
* Embedded Cisco IOS/ASA support
* Supports external collection scripts (such as clogin, jlogin, etc.)
* Currently supports Linux and OSX

##Quickstart:
* Download the sweet binary for your system: https://github.com/AppliedTrust/sweet/releases
* Make it executable: chmod 755 sweet32
* Create a minimal sweet.conf file with device access info:
```
[router1.example.com]
method = cisco
user = sweetuser
pass = SecretPW4sweet
```
* Start sweet with: sweet32 --web sweet.conf
* Check out the web status at: http://localhost:5000

##Usage:
* All command-line flags can also be set in the config file.
* See the sample config for more options: https://github.com/AppliedTrust/sweet/blob/master/sweet-sample.conf
```
  sweet [options] <config>
  sweet -h --help
  sweet --version

Options:
  -w, --workspace <dir>     Specify workspace directory [default: ./workspace].
  -i, --interval <secs>     Collection interval in secs [default: 300].
  -c, --concurrency <num>   Concurrent device collections [default: 30].
  -t, --to <email@addr>     Send change notifications to this email.
  -f, --from <email@addr>   Send change notifications from this email.
  -s, --smtp <host:port>    SMTP server connection info [default: localhost:25].
  --insecure                Accept untrusted SSH device keys.
  --push                    Do a "git push" after committing changed configs.
  --syslog                  Send log messages to syslog rather than stdout.
  --timeout <secs>          Device collection timeout in secs [default: 60].
  --web                     Run an HTTP status server.
  --weblisten <host:port>   Host and port to use for HTTP status server [default: localhost:5000].
  --version                 Show version.
  -h, --help                Show this screen.
```

##Contributors:
* Randy Else: https://appliedtrust.com/randy
* Trent R. Hein: https://appliedtrust.com/trent
* Ned McClain: https://appliedtrust.com/ned

