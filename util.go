package sweet

import (
	"fmt"
	"github.com/mgutz/ansi"
	"log"
	"os"
	"time"
)

//// logging convenience methods
func (Opts *SweetOptions) LogFatal(err error) {
	if Opts.UseSyslog {
		Opts.Syslog.Emerg(err.Error())
	} else {
		log.Println(ansi.Color(err.Error(), "red+b:white"))
	}
	os.Exit(1)
}
func (Opts *SweetOptions) LogErr(message string) {
	if Opts.UseSyslog {
		Opts.Syslog.Err(message)
	} else {
		log.Println(ansi.Color(message, "red+b"))
	}
}
func (Opts *SweetOptions) LogInfo(message string) {
	if Opts.UseSyslog {
		Opts.Syslog.Info(message)
	} else {
		log.Println(ansi.Color(message, "green+b"))
	}
}

// timeAgo formats time to a string
func timeAgo(oldTime time.Time) string {
	var str string
	duration := time.Since(oldTime)
	seconds := int64(duration.Seconds())
	if seconds <= 0 {
		str = "Now"
	} else if seconds < 60 {
		str = fmt.Sprintf("%d seconds", seconds)
	} else if seconds < 120 {
		str = "1 minute"
	} else if seconds < 3600 {
		str = fmt.Sprintf("%d minutes", seconds/60)
	} else if seconds < 7200 {
		str = "1 hour"
	} else if seconds < 86400 {
		str = fmt.Sprintf("%d hours", seconds/(60*60))
	} else if seconds < 86400*2 {
		str = "1 day"
	} else {
		str = fmt.Sprintf("%d days", seconds/(60*60*24))
	}
	return str
}
