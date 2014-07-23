package sweet

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

//// webserver
func RunWebserver(Opts *SweetOptions) {
	if Opts.HttpEnabled == true {
		Opts.LogInfo(fmt.Sprintf("Starting web server on %s", Opts.HttpListen))
		http.Handle("/configs/", http.StripPrefix("/configs/", http.FileServer(http.Dir(Opts.ExecutableDir+string(os.PathSeparator)+Opts.Workspace))))
		http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
			webStaticHandler(w, r, *Opts)
		})
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			webIndexHandler(w, r, *Opts)
		})
		err := http.ListenAndServe(Opts.HttpListen, nil)
		if err != nil {
			Opts.LogFatal(err)
		}
		Opts.LogInfo("Web server started")
	}
}

//// web dashboard
func webIndexHandler(w http.ResponseWriter, r *http.Request, Opts SweetOptions) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	pageTemplate, err := Asset("tmpl/index.html")
	if err != nil {
		Opts.LogFatal(err)
	}

	t, err := template.New("name").Parse(string(pageTemplate))
	if err != nil {
		Opts.LogFatal(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		Opts.LogFatal(err)
	}

	reports := make(map[string]Report, 0)
	for _, device := range Opts.Devices {
		report, _ := getDeviceWebReport(device, Opts)
		reports[device.Hostname] = *report
	}

	data := map[string]interface{}{
		"Title":      "Dashboard",
		"MyHostname": hostname,
		"Devices":    reports,
		"Now":        time.Now().Format("15:04:05 MST"),
	}

	err = t.Execute(w, data)
	if err != nil {
		Opts.LogFatal(err)
	}
}

// Stats for a device
func getDeviceWebReport(device DeviceAccess, Opts SweetOptions) (*Report, error) {
	timeGitInputFormat := "Mon Jan 2 15:04:05 2006 -0700"

	report := new(Report)
	report.Device = device
	report.Web.EnableDiffLink = false
	report.Web.EnableConfLink = false
	report.Web.Class = "danger"
	report.CollectedTime = time.Now()
	report.ChangedTime = time.Now()

	reg, err := regexp.Compile(`\.`)
	if err != nil {
		Opts.LogFatal(err)
	}
	report.Web.CSSID = reg.ReplaceAllString(device.Hostname, "")

	response := make(chan string)
	Opts.ErrorCacheRequests <- &ErrorCacheRequest{Hostname: device.Hostname, Response: response}
	errorStatus := <-response
	if len(errorStatus) > 0 {
		report.StatusMessage = errorStatus
		return report, err
	}

	configStat, err := os.Stat(device.Hostname)
	if err != nil {
		report.StatusMessage = fmt.Sprintf("Config missing for %s", device.Hostname)
		return report, err
	}
	report.CollectedTime = configStat.ModTime()

	lastCommit, err := exec.Command("git", "log", "-n1", device.Hostname).Output()
	if err != nil {
		report.StatusMessage = fmt.Sprintf("Config git log error for %s", device.Hostname)
		return report, err
	}

	parts := strings.Split(string(lastCommit), "\n")
	if len(parts) < 3 {
		report.StatusMessage = "No previous commit found for this config."
		return report, errors.New(report.StatusMessage)
	}
	commitDate := strings.TrimSpace(strings.TrimPrefix(parts[2], "Date:"))
	report.ChangedTime, err = time.Parse(timeGitInputFormat, commitDate)
	if err != nil {
		report.StatusMessage = fmt.Sprintf("Config git time error for %s", device.Hostname)
		return report, err
	}

	report.Web.EnableConfLink = true
	lastCommitHash, err := getPrevDeviceCommit(device.Hostname)
	if err != nil {
		if err.Error() == "No changes since initial commit." {
			report.Web.Class = "active"
		} else {
			report.Web.Class = "warning"
		}
		report.StatusMessage = err.Error()
	} else {
		report.Diff, report.Added, report.Removed, err = getDiff(device, lastCommitHash)
		if err != nil {
			report.StatusMessage = err.Error()
			return report, err
		}
		report.Web.EnableDiffLink = true
		if time.Since(report.ChangedTime) > (recentHours * time.Hour) {
			report.StatusMessage = "No recent changes."
			report.Web.Class = "success"
		} else {
			report.StatusMessage = "Recent changes."
			report.Web.Class = "info"
		}
	}

	return report, nil
}

// webStaticHandler serves embedded static web files (js&css)
func webStaticHandler(w http.ResponseWriter, r *http.Request, Opts SweetOptions) {
	assetPath := r.URL.Path[1:]
	staticAsset, err := Asset(assetPath)
	if err != nil {
		Opts.LogErr(err.Error())
		http.NotFound(w, r)
		return
	}
	headers := w.Header()
	if strings.HasSuffix(assetPath, ".js") {
		headers["Content-Type"] = []string{"application/javascript"}
	} else if strings.HasSuffix(assetPath, ".css") {
		headers["Content-Type"] = []string{"text/css"}
	}
	io.Copy(w, bytes.NewReader(staticAsset))
}

// CollectedTimeFormatted helper for web
func (r Report) CollectedTimeFormatted() string {
	return timeAgo(r.CollectedTime)
}

// ChangedTimeFormatted helper for web
func (r Report) ChangedTimeFormatted() string {
	return timeAgo(r.ChangedTime)
}
