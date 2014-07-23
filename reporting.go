package sweet

import (
	"bytes"
	"errors"
	"fmt"
	"net/smtp"
	"os"
	"os/exec"
	"strings"
)

//
func getLastGlobalCommit() (string, error) {
	lines, err := exec.Command("git", "log", "-n1", "--format=oneline").Output()
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(lines), "\n")
	if len(parts) < 1 {
		return "", errors.New("No commits found!")
	}
	if len(parts[0]) < 40 {
		return "", errors.New("Error finding last global commit hash.")
	}
	hash := parts[0][0:40]
	return hash, nil
}
func getLastDeviceCommit(hostname string) (string, error) {
	lines, err := exec.Command("git", "log", "-n1", "--format=oneline", hostname).Output()
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(lines), "\n")
	if len(parts) < 1 {
		return "", errors.New("No commits for this host.")
	}
	if len(parts[0]) < 40 {
		return "", errors.New("Error finding last commit hash.")
	}
	hash := parts[0][0:40]
	return hash, nil
}
func getPrevDeviceCommit(hostname string) (string, error) {
	lines, err := exec.Command("git", "log", "-n2", "--format=oneline", hostname).Output()
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(lines), "\n")
	if len(parts) < 3 {
		return "", errors.New("No changes since initial commit.")
	}
	if len(parts[1]) < 40 {
		return "", errors.New("Error finding previous commit hash.")
	}
	hash := parts[1][0:40]
	return hash, nil
}

// collect and cleanup diff stats
func getDiff(device DeviceAccess, commitHash string) (string, string, string, error) {
	var diff, added, removed string
	var lines []byte
	var err error
	if len(commitHash) == 40 {
		lines, err = exec.Command("git", "diff", "--numstat", commitHash, device.Hostname).Output()
	} else {
		lines, err = exec.Command("git", "diff", "--numstat", device.Hostname).Output()
	}
	if err != nil {
		return diff, added, removed, err
	}
	if len(lines) < 1 {
		added = "0"
		removed = "0"
	} else {
		fields := strings.Fields(string(lines))
		added = fields[0]
		removed = fields[1]
	}
	diffRaw, err := exec.Command("git", "diff", "-U10", commitHash, device.Hostname).Output()
	if err != nil {
		return diff, added, removed, err
	}
	diffArr := strings.Split(string(diffRaw), "\n")
	if len(diffArr) < 5 {
		return diff, added, removed, errors.New("Diff failed")
	}
	diffArr = diffArr[4:len(diffArr)]
	diff = strings.Join(diffArr, "\n")
	return diff, added, removed, nil
}

//// Handle reporting and notification
func runReporter(Opts SweetOptions) {
	Opts.LogInfo("Starting reporter.")
	changeReport := ""

	lastGlobalCommit, err := getLastGlobalCommit()
	if err != nil {
		Opts.LogFatal(err)
	}
	msg := fmt.Sprintf("Commit ID for this change: %s", lastGlobalCommit)
	changeReport += msg + "\n\n"
	Opts.LogInfo(msg)

	for _, device := range Opts.Devices {
		report, err := getDeviceEmailReport(device, Opts)
		changeReport += fmt.Sprintf("%s: %s\n", device.Hostname, report.StatusMessage)
		if err != nil {
			Opts.LogInfo(fmt.Sprintf("%s: %s", device.Hostname, report.StatusMessage))
		} else {
			Opts.LogInfo(fmt.Sprintf("%s: %s", device.Hostname, report.StatusMessage))
			if len(report.Diff) > 0 {
				Opts.LogInfo("Diff: " + report.Diff)
				changeReport += report.Diff + "\n"
			}
		}
	}
	// send email notification here
	if len(Opts.ToEmail) > 0 && len(Opts.FromEmail) > 0 {
		Opts.LogInfo("Sending notification email")
		hostname, err := os.Hostname()
		if err != nil {
			Opts.LogFatal(err)
		}
		emailSubject := fmt.Sprintf("Change notification from Sweet on %s", hostname)
		err = sendEmail(Opts, emailSubject, changeReport)
		if err != nil {
			Opts.LogErr(fmt.Sprintf("Error sending notification email: %s", err.Error()))
		}
	}
	Opts.LogInfo("Finished reporter.")
}

//// Send an email helper
func sendEmail(Opts SweetOptions, subject, body string) error {
	c, err := smtp.Dial(Opts.SmtpString)
	if err != nil {
		return err
	}
	c.Mail(Opts.FromEmail)
	c.Rcpt(Opts.ToEmail)
	wc, err := c.Data()
	if err != nil {
		return err
	}
	defer wc.Close()
	message := bytes.NewBufferString(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", Opts.ToEmail, subject, body))
	if _, err = message.WriteTo(wc); err != nil {
		return err
	}

	return nil
}

// Email stats for a device
func getDeviceEmailReport(device DeviceAccess, Opts SweetOptions) (*Report, error) {
	report := new(Report)
	report.Device = device
	configStat, err := os.Stat(device.Hostname)
	if err != nil {
		response := make(chan string)
		Opts.ErrorCacheRequests <- &ErrorCacheRequest{Hostname: device.Hostname, Response: response}
		errorStatus := <-response
		if len(errorStatus) > 0 {
			report.StatusMessage = errorStatus
		} else {
			report.StatusMessage = fmt.Sprintf("Config missing for %s", device.Hostname)
		}
		return report, err
	}
	report.CollectedTime = configStat.ModTime()
	lastGlobalCommit, err := getLastGlobalCommit()
	if err != nil {
		response := make(chan string)
		Opts.ErrorCacheRequests <- &ErrorCacheRequest{Hostname: device.Hostname, Response: response}
		errorStatus := <-response
		if len(errorStatus) > 0 {
			report.StatusMessage = errorStatus
		} else {
			report.StatusMessage = err.Error()
		}
		return report, err
	}
	lastDeviceCommit, err := getLastDeviceCommit(device.Hostname)
	if err != nil {
		response := make(chan string)
		Opts.ErrorCacheRequests <- &ErrorCacheRequest{Hostname: device.Hostname, Response: response}
		errorStatus := <-response
		if len(errorStatus) > 0 {
			report.StatusMessage = errorStatus
		} else {
			report.StatusMessage = err.Error()
		}
		return report, err
	}
	if lastGlobalCommit != lastDeviceCommit {
		report.StatusMessage = "No changes."
	} else {
		prevDeviceCommit, err := getPrevDeviceCommit(device.Hostname)
		report.Diff, report.Added, report.Removed, err = getDiff(device, prevDeviceCommit)
		if err != nil {
			response := make(chan string)
			Opts.ErrorCacheRequests <- &ErrorCacheRequest{Hostname: device.Hostname, Response: response}
			errorStatus := <-response
			if len(errorStatus) > 0 {
				report.StatusMessage = errorStatus
			} else {
				report.StatusMessage = err.Error()
			}
			return report, err
		}
		report.StatusMessage = fmt.Sprintf("Changes detected: +%s -%s", report.Added, report.Removed)
	}
	return report, nil
}
