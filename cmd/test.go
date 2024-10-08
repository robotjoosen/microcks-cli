/*
 * Copyright The Microcks Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/microcks/microcks-cli/pkg/config"
	"github.com/microcks/microcks-cli/pkg/connectors"
)

var runnerChoices = map[string]bool{
	"HTTP":             true,
	"SOAP_HTTP":        true,
	"SOAP_UI":          true,
	"POSTMAN":          true,
	"OPEN_API_SCHEMA":  true,
	"ASYNC_API_SCHEMA": true,
	"GRPC_PROTOBUF":    true,
	"GRAPHQL_SCHEMA":   true,
}

type testCommand struct {
}

// NewTestCommand build a new TestCommand implementation
func NewTestCommand() Command {
	return new(testCommand)
}

// Execute implementation of testCommand structure
func (c *testCommand) Execute() {
	var err error

	// Parse subcommand args first.
	if len(os.Args) < 5 {
		fmt.Println("test command require <apiName:apiVersion> <testEndpoint> <runner> args")
		os.Exit(1)
	}

	serviceRef := os.Args[2]
	testEndpoint := os.Args[3]
	runnerType := os.Args[4]

	// Validate presence and values of args.
	if &serviceRef == nil || strings.HasPrefix(serviceRef, "-") {
		fmt.Println("test command require <apiName:apiVersion> <testEndpoint> <runner> args")
		os.Exit(1)
	}
	if &testEndpoint == nil || strings.HasPrefix(testEndpoint, "-") {
		fmt.Println("test command require <apiName:apiVersion> <testEndpoint> <runner> args")
		os.Exit(1)
	}
	if &runnerType == nil || strings.HasPrefix(runnerType, "-") {
		fmt.Println("test command require <apiName:apiVersion> <testEndpoint> <runner> args")
		os.Exit(1)
	}
	if _, validChoice := runnerChoices[runnerType]; !validChoice {
		fmt.Println("<runner> should be one of: HTTP, SOAP, SOAP_UI, POSTMAN, OPEN_API_SCHEMA, ASYNC_API_SCHEMA, GRPC_PROTOBUF, GRAPHQL_SCHEMA")
		os.Exit(1)
	}

	// Then parse flags.
	testCmd := flag.NewFlagSet("test", flag.ExitOnError)

	var microcksURL string
	var keycloakClientID string
	var keycloakClientSecret string
	var waitFor string
	var secretName string
	var filteredOperations string
	var operationsHeaders string
	var oAuth2Context string
	var insecureTLS bool
	var caCertPaths string
	var verbose bool

	testCmd.StringVar(&microcksURL, "microcksURL", "", "Microcks API URL")
	testCmd.StringVar(&keycloakClientID, "keycloakClientId", "", "Keycloak Realm Service Account ClientId")
	testCmd.StringVar(&keycloakClientSecret, "keycloakClientSecret", "", "Keycloak Realm Service Account ClientSecret")
	testCmd.StringVar(&waitFor, "waitFor", "5sec", "Time to wait for test to finish")
	testCmd.StringVar(&secretName, "secretName", "", "Secret to use for connecting test endpoint")
	testCmd.StringVar(&filteredOperations, "filteredOperations", "", "List of operations to launch a test for")
	testCmd.StringVar(&operationsHeaders, "operationsHeaders", "", "Override of operations headers as JSON string")
	testCmd.StringVar(&oAuth2Context, "oAuth2Context", "", "Spec of an OAuth2 client context as JSON string")
	testCmd.BoolVar(&insecureTLS, "insecure", false, "Whether to accept insecure HTTPS connection")
	testCmd.StringVar(&caCertPaths, "caCerts", "", "Comma separated paths of CRT files to add to Root CAs")
	testCmd.BoolVar(&verbose, "verbose", false, "Produce dumps of HTTP exchanges")
	testCmd.Parse(os.Args[5:])

	// Validate presence and values of flags.
	if len(microcksURL) == 0 {
		fmt.Println("--microcksURL flag is mandatory. Check Usage.")
		os.Exit(1)
	}
	if len(keycloakClientID) == 0 {
		fmt.Println("--keycloakClientId flag is mandatory. Check Usage.")
		os.Exit(1)
	}
	if len(keycloakClientSecret) == 0 {
		fmt.Println("--keycloakClientSecret flag is mandatory. Check Usage.")
		os.Exit(1)
	}
	if &waitFor == nil || (!strings.HasSuffix(waitFor, "milli") && !strings.HasSuffix(waitFor, "sec") && !strings.HasSuffix(waitFor, "min")) {
		fmt.Println("--waitFor format is wrong. Applying default 5sec")
		waitFor = "5sec"
	}

	// Collect optional HTTPS transport flags.
	if insecureTLS {
		config.InsecureTLS = true
	}
	if len(caCertPaths) > 0 {
		config.CaCertPaths = caCertPaths
	}
	if verbose {
		config.Verbose = true
	}

	// Compute time to wait in milliseconds.
	var waitForMilliseconds int64 = 5000
	if strings.HasSuffix(waitFor, "milli") {
		waitForMilliseconds, _ = strconv.ParseInt(waitFor[:len(waitFor)-5], 0, 64)
	} else if strings.HasSuffix(waitFor, "sec") {
		waitForMilliseconds, _ = strconv.ParseInt(waitFor[:len(waitFor)-3], 0, 64)
		waitForMilliseconds = waitForMilliseconds * 1000
	} else if strings.HasSuffix(waitFor, "min") {
		waitForMilliseconds, _ = strconv.ParseInt(waitFor[:len(waitFor)-3], 0, 64)
		waitForMilliseconds = waitForMilliseconds * 60 * 1000
	}

	mc := connectors.NewMicrocksClient(microcksURL)
	mc.SetOAuthToken("unauthentifed-token")

	var testResultID string
	testResultID, err = mc.CreateTestResult(serviceRef, testEndpoint, runnerType, secretName, waitForMilliseconds, filteredOperations, operationsHeaders, oAuth2Context)
	if err != nil {
		fmt.Printf("Got error when invoking Microcks client creating Test: %s", err)
		os.Exit(1)
	}
	//fmt.Printf("Retrieve TestResult ID: %s", testResultID)

	// Finally - wait before checking and loop for some time
	time.Sleep(1 * time.Second)

	// Add 10.000ms to wait time as it's now representing the server timeout.
	now := nowInMilliseconds()
	future := now + waitForMilliseconds + 10000

	var success = false
	for nowInMilliseconds() < future {
		testResultSummary, err := mc.GetTestResult(testResultID)
		if err != nil {
			fmt.Printf("Got error when invoking Microcks client check TestResult: %s", err)
			os.Exit(1)
		}
		success = testResultSummary.Success
		inProgress := testResultSummary.InProgress
		fmt.Printf("MicrocksClient got status for test \"%s\" - success: %s, inProgress: %s \n", testResultID, fmt.Sprint(success), fmt.Sprint(inProgress))

		if !inProgress {
			break
		}

		fmt.Println("MicrocksTester waiting for 2 seconds before checking again or exiting.")
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("Full TestResult details are available here: %s/#/tests/%s \n", strings.Split(microcksURL, "/api")[0], testResultID)

	if !success {
		os.Exit(1)
	}
}

func nowInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
