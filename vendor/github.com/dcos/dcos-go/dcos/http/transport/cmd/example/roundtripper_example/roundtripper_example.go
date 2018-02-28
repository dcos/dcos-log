// Copyright 2016 Mesosphere, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dcos/dcos-go/dcos/http/transport"
)

var (
	flagURL       = flag.String("url", "", "URL to query")
	flagIAMConfig = flag.String("iam-config", "", "Path to IAM config")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s -url http://127.0.0.1/system/health/v1 -iam-config /run/dcos/etc/3dt/master_service_account.json\n\n", os.Args[0])
	}
	flag.Parse()
	if *flagURL == "" || *flagIAMConfig == "" {
		flag.Usage()
		os.Exit(1)
	}

	c := &http.Client{}
	rt, err := transport.NewRoundTripper(c.Transport,
		transport.OptionReadIAMConfig(*flagIAMConfig),
		transport.OptionTokenExpire(time.Duration(time.Second*2)))
	if err != nil {
		log.Fatal(err)
	}
	c.Transport = rt

	req, _ := http.NewRequest("GET", *flagURL, nil)

	for {

		resp, err := c.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Expecting return code 200. Got %d", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(body))
		resp.Body.Close()
		time.Sleep(time.Second)
	}
}
