package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const (
	// date format required by date command: ccyymmddHHMMSS.
	// see date(1) manpage.
	dateFormat = "20060102150405"
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
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run date command: %v", err)
		}
	default:
		return fmt.Errorf("setting time on OS '%s' not supported!", runtime.GOOS)
	}

	return nil
}
