package sweet

import (
	"bytes"
	"fmt"
	"net/smtp"
	"os"
)

//// Handle reporting and notification
func runReporter(Opts *SweetOptions) error {
	Opts.LogInfo("Starting reporter.")
	changeReport := ""
	changeDiffs := ""

	// print changes to log
	for _, device := range Opts.Devices {
		stat := Opts.StatusGet(device.Hostname)
		str := ""
		if stat.State == StateSuccess {
			if len(stat.Diffs) < 1 {
				str = fmt.Sprintf("%s: no changes\n", device.Hostname)
			} else {
				str = fmt.Sprintf("%s: changes!\n", device.Hostname)
				for name, d := range stat.Diffs {
					if d.NewFile {
						str += fmt.Sprintf("\t%s: new config\n", name)
					} else {
						str += fmt.Sprintf("\t%s: +%d -%d\n", name, d.Added, d.Removed)
						changeDiffs += fmt.Sprintf("\n---- Diff for %s %s:\n", device.Hostname, name)
						changeDiffs += fmt.Sprintf("%s\n", d.Diff)
					}
				}
			}
		} else {
			str = fmt.Sprintf("%s: error: %s\n", device.Hostname, stat.ErrorMessage)
		}
		changeReport += str
	}

	// Send email notification here
	if len(Opts.ToEmail) > 0 && len(Opts.FromEmail) > 0 {
		Opts.LogInfo(fmt.Sprintf("Sending notification email to %s from %s", Opts.ToEmail, Opts.FromEmail))
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("Error getting my hostname: %s", err.Error())
		}
		emailSubject := fmt.Sprintf("Change notification from Sweet on %s", hostname)
		err = sendEmail(Opts, emailSubject, changeReport+changeDiffs)
		if err != nil {
			return fmt.Errorf("Error sending notification email: %s", err.Error())
		}
	}

	Opts.LogInfo("Finished reporter.")
	return nil
}

//// Send an email helper
func sendEmail(Opts *SweetOptions, subject, body string) error {
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
