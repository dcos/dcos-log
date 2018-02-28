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

package transport

import (
	"crypto/x509"
	"reflect"
	"testing"
)

const (
	CACertPath     = "fixtures/root_ca_cert.pem"
	ServiceAccount = "fixtures/test_service_account.json"
)

func TestLoadCAPool(t *testing.T) {
	caPool, err := loadCAPool(CACertPath)

	if err != nil {
		t.Error("Expected no errors loading CA cert fixutre, got", err)
	}

	if reflect.TypeOf(caPool) != reflect.TypeOf(&x509.CertPool{}) {
		t.Errorf("loadCAPool() returned invalid type, got %T", caPool)
	}

	_, defErr := loadCAPool("fake/cert/path")
	if defErr == nil {
		t.Error("Expected error with bad path, got", defErr)
	}
}

func TestConfigureTLS(t *testing.T) {
	tr, err := configureTLS(CACertPath)

	if err != nil {
		t.Error("Expected no errors, got", err.Error())
	}

	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected skip verify to be false, got true")
	}

	noVerifyTr, noVerifyErr := configureTLS("")

	if noVerifyErr != nil {
		t.Error("Expected no errors, got", noVerifyErr.Error())
	}

	if !noVerifyTr.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected skip verify to be true, got false")
	}
}
