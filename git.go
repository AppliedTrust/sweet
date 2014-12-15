package sweet

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//
func commitChanges(Opts *SweetOptions) error {
	statusText, err := exec.Command("git", "status", "-s").Output()
	if err != nil {
		return fmt.Errorf("Git status error: %s", err.Error())
	}
	if len(statusText) > 0 {
		_, err = exec.Command("git", "add", ".").Output()
		if err != nil {
			return fmt.Errorf("Git add error: %s", err.Error())
		}

		commitMsg := "Sweet commit:\n" + string(statusText)
		_, err = exec.Command("git", "commit", "-a", "-m", commitMsg).Output()
		if err != nil {
			return fmt.Errorf("Git commit error: %s", err.Error())
		}
		if Opts.GitPush == true {
			_, err = exec.Command("git", "push").Output()
			if err != nil {
				Opts.LogErr(fmt.Sprintf("Git push failed, continuing anyway: %s", err.Error()))
			}
		}
		Opts.LogInfo(fmt.Sprintf("Committed changes to git."))
	} else {
		Opts.LogInfo("No changes to commit.")
	}
	return nil
}

// collect and cleanup diff stats
func updateDiffs(Opts *SweetOptions) error {
	for _, device := range Opts.Devices {
		stat := Opts.Status.Get(device.Hostname)
		stat.Diffs = make(map[string]ConfigDiff)
		for name, _ := range stat.Configs {
			diff := ConfigDiff{}
			fileName := device.Hostname + "-" + cleanName(name)
			s, err := exec.Command("git", "status", "-s", fileName).Output()
			if err != nil {
				return err
			}
			if strings.HasPrefix(string(s), "??") { // new file
				diff.NewFile = true
				stat.Diffs[name] = diff
			} else if strings.HasPrefix(string(s), " M") { // existing file w/changes
				diffRaw, err := exec.Command("git", "diff", "-U4", fileName).Output()
				if err != nil {
					return err
				}
				if len(diffRaw) > 0 {
					diffArr := strings.Split(string(diffRaw), "\n")
					if len(diffArr) > 4 {
						diffArr = diffArr[4:len(diffArr)]
					}
					diff.Diff = strings.Join(diffArr, "\n")
				}
				lines, err := exec.Command("git", "diff", "--numstat", fileName).Output()
				if err != nil {
					return err
				}
				fields := strings.Fields(string(lines))
				diff.Added, err = strconv.Atoi(fields[0])
				if err != nil {
					return err
				}
				diff.Removed, err = strconv.Atoi(fields[1])
				if err != nil {
					return err
				}

				stat.Diffs[name] = diff
			} else if len(string(s)) < 1 {
				// no changes in this file
			} else {
				return fmt.Errorf("unexpected git diff response: %s", s)
			}
		}
		Opts.Status.Set(stat)
	}
	return nil
}
