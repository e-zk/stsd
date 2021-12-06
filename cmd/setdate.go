package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

const (
	// date format required by date command: 'ccyymmddHHMM.SS'.
	// see date(1) manpage.
	dateFormat = "200601021504.05"
)

// depending on OS set the date
// NOTE currently works by shelling out to the 'date' command which is not
//      ideal. however, this does mean all operating systems with POSIX
//      compliant date(1) are supported.
func setOsDate(date string, os string) error {
	// parse date in current locale
	zone := time.Now().Location()
	t, err := time.ParseInLocation(time.RFC1123, date, zone)
	if err != nil {
		return fmt.Errorf("failed to parse given date: %v", err)
	}

	// convert to local 'ccyymmddHHMM.SS' format
	dateCmdTime := t.Local().Format(dateFormat)

	var cmd *exec.Cmd

	// NOTE this is UNTESTED on macos, freebsd, dflybsd, linux.
	//      however, all the systems' date command follows POSIX
	// NOTE {free,dfly}bsd + macos do not support -a (adjtime).
	switch os {
	// net+openbsd support adjtime(2)
	case "netbsd":
		fallthrough
	case "openbsd":
		cmd = exec.Command("date", "-a", dateCmdTime)

	// the rest, do not
	case "freebsd":
		fallthrough
	case "dragonflybsd":
		fallthrough
	case "darwin":
		fallthrough
	case "linux":
		cmd = exec.Command("date", dateCmdTime)
	default:
		return fmt.Errorf("setting time on OS '%s' not supported!", os)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error attaching stderr pipe: %v", err)
	}

	log.Printf("running: '%s'", cmd.String())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run date command: %v", err)
	}

	// read stderr pipe
	stderrString, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to run date command: %v. stderr: %s", err, stderrString)
	}

	return nil
}
