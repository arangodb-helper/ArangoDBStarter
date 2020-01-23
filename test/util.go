//
// DISCLAIMER
//
// Copyright 2017 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
//

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arangodb-helper/arangodb/client"
	shell "github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
)

const (
	ctrlC                     = "\u0003"
	whatCluster               = "cluster"
	whatSingle                = "single server"
	whatResilientSingle       = "resilient single server"
	testModeProcess           = "localprocess"
	testModeDocker            = "docker"
	starterModeCluster        = "cluster"
	starterModeSingle         = "single"
	starterModeActiveFailover = "activefailover"
	portIncrement             = 10
)

var (
	isVerbose    bool
	isEnterprise bool
	testModes    []string
	starterModes []string
)

func init() {
	isVerbose = strings.TrimSpace(os.Getenv("VERBOSE")) != ""
	isEnterprise = strings.TrimSpace(os.Getenv("ENTERPRISE")) != ""
	testModes = strings.Split(strings.TrimSpace(os.Getenv("TEST_MODES")), ",")
	if len(testModes) == 1 && testModes[0] == "" {
		testModes = nil
	}
	starterModes = strings.Split(strings.TrimSpace(os.Getenv("STARTER_MODES")), ",")
	if len(starterModes) == 1 && starterModes[0] == "" {
		starterModes = nil
	}
}

func needTestMode(t *testing.T, testMode string) {
	for _, x := range testModes {
		if x == testMode {
			return
		}
	}
	if len(testModes) == 0 {
		return
	}
	t.Skipf("Test mode '%s' not set", testMode)
}

func needStarterMode(t *testing.T, starterMode string) {
	for _, x := range starterModes {
		if x == starterMode {
			return
		}
	}
	if len(starterModes) == 0 {
		return
	}
	t.Skipf("Starter mode '%s' not set, have %v", starterMode, starterModes)
}

func needEnterprise(t *testing.T) {
	if isEnterprise {
		return
	}
	t.Skip("Enterprise is not available")
}

// Spawn a command an return its process and expand envs.
func Spawn(t *testing.T, command string) *SubProcess {
	return SpawnWithExpand(t, command, true)
}

// Spawn a command an return its process with optionally expanded envs.
func SpawnWithExpand(t *testing.T, command string, expand bool) *SubProcess {
	command = strings.TrimSpace(command)
	if expand {
		command = os.ExpandEnv(command)
	}
	t.Logf("Executing command: %s", command)
	args, err := shell.Split(command)
	if err != nil {
		t.Fatal(describe(err))
	}
	if isVerbose {
		t.Log(args, len(args))
	}
	p, err := NewSubProcess(args[0], args[1:]...)
	if err != nil {
		t.Fatal(describe(err))
	}
	if err := p.Start(); err != nil {
		p.Close()
		t.Fatal(describe(err))
	}
	return p
}

// SetUniqueDataDir creates a temp dir and sets the DATA_DIR environment variable to it.
func SetUniqueDataDir(t *testing.T) string {
	dataDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(describe(err))
	}
	os.Setenv("DATA_DIR", dataDir)
	return dataDir
}

type waitUntilReadyResult struct {
	Ready    bool
	TimeSpan time.Duration
	Message  string
}

// WaitUntilStarterReady waits until all given starter processes have reached the "Your cluster is ready state"
func WaitUntilStarterReady(t *testing.T, what string, requiredGoodResults int, starters ...*SubProcess) bool {
	results := make(chan waitUntilReadyResult, len(starters))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for index, starter := range starters {
		starter := starter // Used in nested function
		id := fmt.Sprintf("starter-%d", index+1)
		go func() {
			started := time.Now()
			if err := starter.ExpectTimeout(ctx, time.Minute*3, regexp.MustCompile(fmt.Sprintf("Your %s can now be accessed with a browser at", what)), id); err != nil {
				timeSpan := time.Since(started)
				results <- waitUntilReadyResult{
					Ready:    false,
					TimeSpan: timeSpan,
					Message:  fmt.Sprintf("Starter is not ready in time (after %s): %s", timeSpan, describe(err)),
				}
			} else {
				results <- waitUntilReadyResult{
					Ready: true,
				}
			}
		}()
	}
	okCount := 0
	errorCount := 0
	errorMessages := make([]string, 0, len(starters))
	for result := range results {
		if result.Ready {
			okCount++
		} else {
			errorCount++
			errorMessages = append(errorMessages, result.Message)
		}
		if okCount >= requiredGoodResults {
			return true
		}
		if okCount+errorCount == len(starters) {
			break
		}
	}
	if os.Getenv("DEBUG_CLUSTER") == "interactive" {
		// Halt forever
		fmt.Println("Cluster not ready in time, halting forever for debugging")
		for {
			time.Sleep(time.Hour)
		}
	}
	for _, msg := range errorMessages {
		t.Error(msg)
	}
	return false
}

// SendIntrAndWait stops all all given starter processes by sending a Ctrl-C into it.
// It then waits until the process has terminated.
func SendIntrAndWait(t *testing.T, starters ...*SubProcess) bool {
	g := sync.WaitGroup{}
	result := true
	for _, starter := range starters {
		starter := starter // Used in nested function
		g.Add(1)
		go func() {
			defer g.Done()
			if err := starter.WaitTimeout(time.Second * 300); err != nil {
				result = false
				t.Errorf("Starter is not stopped in time: %s", describe(err))
			}
		}()
	}
	time.Sleep(time.Second)
	for _, starter := range starters {
		starter.SendIntr()
		//starter.Send(ctrlC)
	}
	g.Wait()
	return result
}

// describe returns a string description of the given error.
func describe(err error) string {
	if err == nil {
		return "nil"
	}
	cause := errors.Cause(err)
	c, _ := json.Marshal(cause)
	cStr := fmt.Sprintf("%#v (%s)", cause, string(c))
	if cause.Error() != err.Error() {
		return fmt.Sprintf("%v caused by %v", err, cStr)
	} else {
		return cStr
	}
}

// NewStarterClient creates a new starter API instance for the given endpoint, failing the test on errors.
func NewStarterClient(t *testing.T, endpoint string) client.API {
	ep, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("Failed to parse starter endpoint: %s", describe(err))
	}
	c, err := client.NewArangoStarterClient(*ep)
	if err != nil {
		t.Fatalf("Failed to create starter client: %s", describe(err))
	}
	return c
}

// ShutdownStarter calls the starter the shutdown via the HTTP API.
func ShutdownStarter(t *testing.T, endpoint string) {
	c := NewStarterClient(t, endpoint)
	if err := c.Shutdown(context.Background(), false); err != nil {
		t.Errorf("Shutdown failed: %s", describe(err))
	}
	WaitUntilStarterGone(t, endpoint)
}

// WaitUntilStarterGone waits until the starter at given endpoint no longer responds to queries.
func WaitUntilStarterGone(t *testing.T, endpoint string) {
	c := NewStarterClient(t, endpoint)
	failures := 0
	for {
		if _, err := c.Version(context.Background()); err != nil {
			// Version request failed
			failures++
		} else {
			failures = 0
		}
		if failures > 2 {
			// Several failures, we assume the starter is really gone now
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func createEnvironmentStarterOptions(skipDockerImage ...bool) string {
	result := []string{"--starter.debug-cluster"}
	if image := os.Getenv("ARANGODB"); image != "" {
		if len(skipDockerImage) == 0 || !skipDockerImage[0] {
			result = append(result, fmt.Sprintf("--docker.image=%s", image))
		}
	}
	return strings.Join(result, " ")
}

func createLicenseKeyOption() string {
	if license := os.Getenv("ARANGO_LICENSE_KEY"); license != "" {
		return "-e ARANGO_LICENSE_KEY=" + license
	}
	return ""
}
