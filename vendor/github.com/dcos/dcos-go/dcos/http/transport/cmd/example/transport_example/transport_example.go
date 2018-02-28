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
	url        = flag.String("url", "", "URL to query")
	iamConfig  = flag.String("iam-config", "", "Path to IAM config")
	caCertPath = flag.String("ca-cert-path", "", "Path to CA certificate")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s -url http://127.0.0.1/system/health/v1 -ca-cert-path /run/dcos/etc/ca_cert.pem -iam-config /run/dcos/etc/3dt/master_service_account.json\n\n", os.Args[0])
	}
	flag.Parse()
	if *url == "" || *iamConfig == "" || *caCertPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	tr, err := transport.NewTransport(transport.OptionCaCertificatePath(*caCertPath), transport.OptionIAMConfigPath(*iamConfig))
	if err != nil {
		panic(err)
	}

	c := http.Client{
		Transport: tr,
	}

	for {
		resp, err := c.Get(*url)
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
