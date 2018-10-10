#!/bin/bash
# This script performs tests against the dcos-metrics project, specifically:
#
#   * gofmt         (https://golang.org/cmd/gofmt)
#   * goimports     (https://godoc.org/cmd/goimports)
#   * golint        (https://github.com/golang/lint)
#   * go vet        (https://golang.org/cmd/vet)
#   * test coverage (https://blog.golang.org/cover)
#
# It outputs test and coverage reports in a way that Jenkins can understand,
# with test results in JUnit format and test coverage in Cobertura format.
# The reports are saved to build/$SUBDIR/{test-reports,coverage-reports}/*.xml 
#
set -e
set -o pipefail
export PATH="${GOPATH}/bin:${PATH}"

PACKAGES="$(go list ./... | grep -v /vendor/)"
SUBDIRS=$(go list -f {{.Dir}} ./... | grep -v /vendor/)
SOURCE_DIR=$(git rev-parse --show-toplevel)
BUILD_DIR="${SOURCE_DIR}/build"


function logmsg {
    echo -e "\n\n*** $1 ***\n"
}

function _gofmt {
    logmsg "Running 'gofmt' ..."
    test -z "$(gofmt -l -d ${SUBDIRS} | tee /dev/stderr)"
}


function _goimports {
    logmsg "Running 'goimports' ..."
    go get -u golang.org/x/tools/cmd/goimports
    test -z "$(gofmt -l -d ${SUBDIRS} | tee /dev/stderr)"
}


function _golint {
    logmsg "Running 'go lint' ..."
    go get -u github.com/golang/lint/golint
    for pkg in $PACKAGES; do
        golint -set_exit_status $pkg
    done
}


function _govet {
    logmsg "Running 'go vet' ..."
    go vet ${PACKAGES}
}


function _unittest_with_coverage {
    local covermode="atomic"
    logmsg "Running unit tests ..."

    go get -u github.com/jstemmer/go-junit-report
    go get -u github.com/smartystreets/goconvey
    go get -u golang.org/x/tools/cmd/cover
    go get -u github.com/axw/gocov/...
    go get -u github.com/AlekSi/gocov-xml

    # We can't' use the test profile flag with multiple packages. Therefore,
    # run 'go test' for each package, and concatenate the results into
    # 'profile.cov'.
    mkdir -p ${BUILD_DIR}/{test-reports,coverage-reports}
    echo "mode: ${covermode}" > ${BUILD_DIR}/coverage-reports/profile.cov

    for import_path in ${PACKAGES}; do
        package=$(basename ${import_path})

        go test -v -race -covermode=$covermode                                   \
            -coverprofile="${BUILD_DIR}/coverage-reports/profile_${package}.cov" \
            $import_path | tee /dev/stderr                                       \
            | go-junit-report > "${BUILD_DIR}/test-reports/${package}-report.xml"

    done

    # Concatenate per-package coverage reports into a single file.
    for f in ${BUILD_DIR}/coverage-reports/profile_*.cov; do
        tail -n +2 ${f} >> ${BUILD_DIR}/coverage-reports/profile.cov
        rm $f
    done

    go tool cover -func ${BUILD_DIR}/coverage-reports/profile.cov
    gocov convert ${BUILD_DIR}/coverage-reports/profile.cov \
        | gocov-xml > "${BUILD_DIR}/coverage-reports/coverage.xml"
}


# Main.
function main {
    _gofmt
    _goimports
    _golint
    _govet
    _unittest_with_coverage
}

main
