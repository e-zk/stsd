package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"math/rand"
	"net"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

const (
	defaultTorProxy  = "socks5://127.0.0.1:9050"
	defaultPoolFile  = "/etc/stsd_pool"
	defaultSocket    = "/var/run/stsd.sock"
	defaultDateCmd   = "/bin/date"
	defaultChildUser = "_stsd"
	socketMsgLen     = 29
	minSleep         = 64
	maxSleep         = 1024
	usageText        = `secure time sync daemon
usage: stsd [--date-cmd=path] [--user=username] [--pool-file=file]
            [--use-proxy=proxy | --use-tor[=proxy]]
where:
  --date-cmd=path    absolute path to date(1) command (default: '/bin/date').
  --user=username    user to run child process as (default: '_stsd').
  --pool-file=file   use the specified pool file (default: '/etc/stsd_pool').
  --use-proxy=proxy  proxy network requests through 'proxy' url.
  --use-tor          use tor for network requests. favours onion addresses
                     from the pool file. tor's proxy url can be configured
                     by passing as an argument flag: '--use-tor=proxy'
                     (default tor proxy url: 'socks5://localhost:9050').
`
)

var (
	originalArgs  []string
	torProxy      torFlag
	useProxy      string
	poolFile      string
	pname         string
	dateCmdPath   string
	childProcUser string
	childProcAttr *syscall.ProcAttr
)

// Custom flag
type torFlag struct {
	proxyUrl string
}

func (t *torFlag) Set(x string) error {
	t.proxyUrl = x
	return nil
}
func (t *torFlag) String() string {
	return t.proxyUrl
}
func (t *torFlag) IsBoolFlag() bool {
	return true
}

// Sleep for random amount of time
func randomSleep() {
	sleepTime := time.Duration(minSleep + rand.Int63n(maxSleep-minSleep+1))
	log.Printf("sleeping for %v\n", sleepTime*time.Second)
	time.Sleep(sleepTime * time.Second)
}

// Check program is running as root
func checkRoot() (bool, error) {
	// check if we're running as root
	curUser, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("error getting user information: %v", err)
	}

	uid, err := strconv.Atoi(curUser.Uid)
	if err != nil {
		return false, fmt.Errorf("error parsing uid: %v", err)
	}

	if uid != 0 {
		return false, nil
	}

	return true, nil
}

// Update system date
// 1. Fork child process
// 2. Wait for data from the given socket listener
// 3. Set received date
func updateDate(l net.Listener) {
	// update arguments for network process
	newArgs := append(originalArgs, "-P=network")

	cid, err := syscall.ForkExec(newArgs[0], newArgs, childProcAttr)
	if err != nil {
		log.Fatalf("error forking: %v", err)
	}
	log.Printf("forked network process with pid: %d", cid)

	// get child process info
	var cproc *os.Process
	if cproc, err = os.FindProcess(cid); err != nil {
		log.Fatalf("error getting child process information: %v", err)
	}

	// accept connection on listener
	conn, err := l.Accept()
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// read date buffer from socket
	dateBuf := make([]byte, socketMsgLen)
	n, err := conn.Read(dateBuf)
	if n != socketMsgLen {
		log.Fatalf("error: socket content has incorrect length")
	}
	if err != nil {
		log.Fatalf("error reading from socket: %v", err)
	}
	log.Printf("got date: %v", string(dateBuf))

	// set os date
	err = setOsDate(string(dateBuf), dateCmdPath, runtime.GOOS)
	if err != nil {
		log.Fatalf("failed to set date: %v", err)
	}

	// wait for child process to exit
	cproc.Wait()

	// close connection
	conn.Close()
}

func init() {
	rand.Seed(time.Now().Unix())

	// save original arguments before parsing
	originalArgs = os.Args

	// setup flags
	flag.Var(&torProxy, "use-tor", "use tor")
	flag.StringVar(&useProxy, "use-proxy", "", "")
	flag.StringVar(&poolFile, "pool-file", defaultPoolFile, "")
	flag.StringVar(&childProcUser, "user", defaultChildUser, "")
	flag.StringVar(&dateCmdPath, "date-cmd", defaultDateCmd, "")

	// this is the not-so-secret flag that tells stsd that it needs to act
	// as the networking process.
	flag.StringVar(&pname, "P", "", "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageText)
	}
	flag.Parse()
}

func main() {
	// setup syslog
	syslogger, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "stsd")
	if err != nil {
		log.Fatalf("error attaching to syslog: %v", err)
	}
	log.SetFlags(0)
	log.SetOutput(syslogger)

	// setup attributes for child process
	childUser, err := user.Lookup(childProcUser)
	if err != nil {
		log.Fatalf("error getting child user info: %v", err)
	}
	uid, err := strconv.ParseUint(childUser.Uid, 10, 32)
	if err != nil {
		log.Fatalf("error converting uid to int: %v", err)
	}
	childProcAttr = &syscall.ProcAttr{
		Sys: &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
			},
			Setsid: true,
		},
	}

	// if passed w/o an argument set it to the default proxy url
	if torProxy.String() == "true" {
		torProxy.Set(defaultTorProxy)
	}

	// if both --use-proxy and --use-tor are set; throw an error
	if torProxy.String() != "" && useProxy != "" {
		log.Fatalf("error: cannot use --use-tor option with --use-proxy.")
	}

	// remove old socket
	os.RemoveAll(defaultSocket)
	syscall.Umask(0117)

	// run the network process logic if -P has been specified as 'network'
	if pname == "network" {
		NetworkLogic()
	} else {
		isRoot, err := checkRoot()
		if err != nil {
			log.Fatalf("%v", err)
		}

		if !isRoot {
			log.Fatal("error: stsd must be run as root")
		}

		// listen on socket
		l, err := net.Listen("unixpacket", defaultSocket)
		if err != nil {
			log.Fatalf("error opening listener on socket: %v", err)
		}

		// fork
		// wait for connection from child via socket
		// read date from socket
		// set system date
		for {
			updateDate(l)

			// sleep for random amount of time
			randomSleep()
		}
		l.Close()
	}
}
