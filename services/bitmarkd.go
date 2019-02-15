// Copyright (c) 2014-2015 Bitmark Inc.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file

package services

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitmark-inc/bitmark-node/config"
	"github.com/bitmark-inc/bitmark-node/fault"
	"github.com/bitmark-inc/bitmark-node/utils"
	"github.com/bitmark-inc/bitmarkd/account"
	"github.com/bitmark-inc/logger"
)

var (
	ErrBitmarkdIsNotRunning = fault.InvalidError("Bitmarkd is not running")
	ErrBitmarkdIsRunning    = fault.InvalidError("Bitmarkd is running")
)

type Bitmarkd struct {
	sync.RWMutex
	initialised bool
	log         *logger.L
	rootPath    string
	configFile  string
	network     string
	process     *os.Process
	running     bool // determine whether the process is running
	started     bool // determine whether the service is started
	cmdErr      string
	ModeStart   chan bool
	localIP     string
}

var client = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func NewBitmarkd(localIP string) *Bitmarkd {
	return &Bitmarkd{
		localIP: localIP,
	}
}

func (bitmarkd *Bitmarkd) GetPath() string {
	return bitmarkd.rootPath
}

func (bitmarkd *Bitmarkd) GetNetwork() string {
	if len(bitmarkd.network) == 0 {
		return "bitmark"
	}
	return bitmarkd.network
}

func (bitmarkd *Bitmarkd) Initialise(rootPath string) error {
	bitmarkd.Lock()
	defer bitmarkd.Unlock()

	if bitmarkd.initialised {
		return fault.ErrAlreadyInitialised
	}

	bitmarkd.rootPath = rootPath

	bitmarkd.log = logger.New("service-bitmarkd")

	bitmarkd.started = false
	bitmarkd.running = false
	bitmarkd.ModeStart = make(chan bool, 1)

	// all data initialised
	bitmarkd.initialised = true

	bitmarkd.initLoop()
	return nil
}

func (bitmarkd *Bitmarkd) Finalise() error {
	bitmarkd.Lock()
	defer bitmarkd.Unlock()

	if !bitmarkd.initialised {
		return fault.ErrNotInitialised
	}

	bitmarkd.initialised = false
	bitmarkd.Stop()
	return nil
}

func (bitmarkd *Bitmarkd) IsRunning() bool {
	return bitmarkd.running
}

func (bitmarkd *Bitmarkd) Status() map[string]interface{} {
	return map[string]interface{}{
		"started": bitmarkd.started,
		"running": bitmarkd.running,
		"error":   bitmarkd.cmdErr,
	}
}

func (bitmarkd *Bitmarkd) Run(args interface{}, shutdown <-chan struct{}) {
loop:
	for {
		select {

		case <-shutdown:
			break loop
		case start := <-bitmarkd.ModeStart:
			if start {
				bitmarkd.Start()
			} else {
				bitmarkd.Stop()
			}
		}

	}
	close(bitmarkd.ModeStart)
}

func (bitmarkd *Bitmarkd) SetNetwork(network string) {
	if bitmarkd.started {
		bitmarkd.Stop()
	}
	bitmarkd.network = network
	switch network {
	case "testing":
		bitmarkd.configFile = filepath.Join(bitmarkd.rootPath, "testing/bitmarkd.conf")
	case "bitmark":
		fallthrough
	default:
		bitmarkd.configFile = filepath.Join(bitmarkd.rootPath, "bitmark/bitmarkd.conf")
	}
}

func (bitmarkd *Bitmarkd) Start() error {
	if bitmarkd.started {
		bitmarkd.log.Errorf("Start bitmarkd failed: %v", ErrBitmarkdIsRunning)
		return ErrBitmarkdIsRunning
	}

	// Check bitmarkConfigFile exists
	bitmarkd.log.Infof("bitmark config file: %s\n", bitmarkd.configFile)
	if !utils.EnsureFileExists(bitmarkd.configFile) {
		bitmarkd.log.Errorf("Start bitmarkd failed: %v", fault.ErrNotFoundConfigFile)
		return fault.ErrNotFoundConfigFile
	}

	nodeConfig := config.New()
	bitmarkd.started = true

	go func() {
		var runCounter uint64
		for bitmarkd.started {
			// start bitmarkd as sub process
			configs, err := nodeConfig.Get()
			if err != nil {
				bitmarkd.log.Errorf("Can not get the latest node config: %s", err.Error())
			}
			btcAddr := os.Getenv("BTC_ADDR")
			ltcAddr := os.Getenv("LTC_ADDR")
			if v, ok := configs["btcAddr"]; ok && v != "" {
				btcAddr = v
			}
			if v, ok := configs["ltcAddr"]; ok && v != "" {
				ltcAddr = v
			}

			if runCounter < 65535 {
				runCounter++
			}

			cmd := exec.Command("bitmarkd", "--config-file="+bitmarkd.configFile)
			cmd.Env = []string{
				fmt.Sprintf("CONTAINER_IP=%s", bitmarkd.localIP),
				fmt.Sprintf("PUBLIC_IP=%s", os.Getenv("PUBLIC_IP")),
				fmt.Sprintf("BTC_ADDR=%s", btcAddr),
				fmt.Sprintf("LTC_ADDR=%s", ltcAddr),
			}
			// start bitmarkd as sub process
			stderr, err := cmd.StderrPipe()
			if err != nil {
				bitmarkd.log.Errorf("Error: %v", err)
				continue
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				bitmarkd.log.Errorf("Error: %v", err)
				continue
			}
			if err := cmd.Start(); nil != err {
				continue
			}

			bitmarkd.process = cmd.Process
			bitmarkd.log.Infof("process id: %d", cmd.Process.Pid)
			stdeReader := bufio.NewReader(stderr)
			stdoReader := bufio.NewReader(stdout)

			go func() {
				for {
					stde, err := stdeReader.ReadString('\n')
					if nil != err {
						bitmarkd.log.Errorf("Error: %v", err)
						return
					}
					bitmarkd.log.Errorf("bitmarkd stderr: %q", stde)
					bitmarkd.cmdErr = stde
				}
			}()

			go func() {
				for {
					stdo, err := stdoReader.ReadString('\n')
					if nil != err {
						bitmarkd.log.Errorf("Error: %v", err)
						return
					}
					if strings.HasPrefix(stdo, "CURVE I:") {
						continue
					}
					bitmarkd.log.Infof("bitmarkd stdout: %q", stdo)
					bitmarkd.cmdErr = ""
				}
			}()
			bitmarkd.running = true
			if err := cmd.Wait(); nil != err {
				if bitmarkd.started {
					bitmarkd.log.Errorf("bitmarkd has terminated unexpectedly. failed: %v", err)
					delayTime := 5 * runCounter * runCounter
					bitmarkd.log.Errorf("bitmarkd will be restarted in  %v second...", delayTime)
					time.Sleep(time.Duration(delayTime) * time.Second)
				} else {
					bitmarkd.cmdErr = ""
				}
			}
			bitmarkd.process = nil
			bitmarkd.running = false
		}
	}()
	// wait for 1 second if cmd has no error then return nil
	time.Sleep(time.Second * 1)
	return nil

}

func (bitmarkd *Bitmarkd) Stop() error {
	if !bitmarkd.running {
		bitmarkd.log.Errorf("Stop bitmarkd failed: %v", ErrBitmarkdIsNotRunning)
		return ErrBitmarkdIsNotRunning
	}

	bitmarkd.log.Infof("Stop bitmarkd. PID: %d", bitmarkd.process.Pid)

	if bitmarkd.process == nil {
		return nil
	}

	if bitmarkd.started {
		if err := bitmarkd.process.Signal(os.Interrupt); nil != err {
			bitmarkd.log.Errorf("Send sigint to bitmarkd failed: %v", err)
			return err
		}
	} else {
		if err := bitmarkd.process.Signal(os.Kill); nil != err {
			bitmarkd.log.Errorf("Send sigkill to bitmarkd failed: %v", err)
			return err
		}
	}
	bitmarkd.started = false
	return nil
}

func (bitmarkd *Bitmarkd) initLoop() {
	go func() {
		for true {
			time.Sleep(15 * time.Second)
			if bitmarkd.running {
				// Request bitmarkd details
				resp, err := client.Get("https://127.0.0.1:2131/bitmarkd/details")
				if err != nil {
					fmt.Println("unable to get bitmark info")
					continue
				}
				defer resp.Body.Close()
				bb := bytes.Buffer{}
				io.Copy(&bb, resp.Body)

				if resp.StatusCode != http.StatusOK {
					fmt.Println("unable to get bitmark info. message: %s", bb.String())
					continue
				}

				var reply DetailReply
				d := json.NewDecoder(&bb)

				if err := d.Decode(&reply); err != nil {
					fmt.Println("fail to read bitmark info response. error: %s\n", err.Error())
					continue
				}

				t, _ := time.ParseDuration(reply.Uptime)
				reply.Uptime = t.Round(time.Second).String()
				fmt.Println("Blocks: %s", reply.Blocks)

				// Post info to server
				seedFile := filepath.Join(bitmarkd.rootPath, bitmarkd.network, "proof.sign")
				f, err := os.Open(seedFile)
				if err != nil {
					fmt.Println("fail to open seed file from: %s", seedFile)
					continue
				}
				defer f.Close()

				var buf bytes.Buffer
				_, err = io.Copy(&buf, f)
				if err != nil {
					fmt.Println("can not read config file")
					continue
				}

				seed := strings.Trim(strings.Split(buf.String(), ":")[1], "\n")
				a, err := account.PrivateKeyFromBase58Seed(seed)
				if err != nil {
					fmt.Println("unable to get your account. error: %s", err.Error())
					continue
				}

				data := url.Values{"id": {a.Account().String()}, "ip": {os.Getenv("PUBLIC_IP")}, "height": {strconv.FormatUint(reply.Blocks.Local, 10)}}
				body := strings.NewReader(data.Encode())
				dest := os.Getenv("SERVER_IP") + "api/update"
				resp, err = http.Post(dest, "application/x-www-form-urlencoded", body)
				if err != nil {
					fmt.Println("unable to post docker info")
					continue
				}
				defer resp.Body.Close()
			}
		}
	}()
}
