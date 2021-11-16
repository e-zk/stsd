package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"time"
)

const (
	// date format required by date command: ccyymmddHHMMSS.
	// see date(1) manpage.
	dateFormat = "200601021504.05"
)

// depending on OS set the date
// TODO currently works by shelling out to `date` which is not ideal
func setOsDate(date string, forceOs string) error {
	os := runtime.GOOS

	if forceOs != "" {
		os = forceOs
	}

	// parse date in current locale
	zone := time.Now().Location()
	t, err := time.ParseInLocation(time.RFC1123, date, zone)
	if err != nil {
		return fmt.Errorf("failed to parse given date: %v", err)
	}

	// convert to local ccyymmddHHMMSS
	dateCmdTime := t.Local().Format(dateFormat)

	switch os {
	// TODO this is UNTESTED on macos, freebsd, dflybsd, linux.
	// all the following systems' date command follows POSIX
	// so the command is the same for all of them.
	// NOTE {free,dfly}bsd + macos do not support -a (adjtime).
	case "darwin":
		fallthrough
	case "freebsd":
		fallthrough
	case "dragonflybsd":
		fallthrough
	case "openbsd":
		fallthrough
	case "linux":
		cmd := exec.Command("date", dateCmdTime)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("error attaching stderr pipe: %v", err)
		}

		log.Printf("running: '%s'", cmd.String())

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to run date command: %v", err)
		}

		stderrString, _ := io.ReadAll(stderr)

		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("failed to run date command: %v\nstderr: %s", err, stderrString)
		}
	default:
		return fmt.Errorf("setting time on OS '%s' not supported!", runtime.GOOS)
	}

	return nil
}
