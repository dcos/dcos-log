function logmsg($msg)
{
    Write-Output("")
    Write-Output("*** " + $msg + " ***")
}

function fastfail($msg)
{
    if ($LASTEXITCODE -ne 0)
    {
        logmsg($msg)
        exit -1
    }
}
function fastfailpipe {
  Process  {
  fastfail("hello")
  $_
  }
}

function _gofmt()
{
    logmsg("Running 'gofmt' ...")
    $text = & gofmt -d -l $SUBDIRS
    if ($text.length -ne 0)
    {
        Write-Output($text)
        exit -1
    }
}
function _goimports()
{
    logmsg("Running 'goimports' ...")
    & go get -u golang.org/x/tools/cmd/goimports
    fastfail("failed to get goimports")

    $text = & goimports -d -l $SUBDIRS
    
    if ($text.length -ne 0)
    {
        Write-Output($text)
        exit -1
    }
}

function _golint()
{
    logmsg("Running 'golint' ...")
    & go get -u github.com/golang/lint/golint

    $text = & golint -set_exit_status  $PACKAGES
    fastfail("failed to run golint: $text")
    
    if ($text.length -ne 0)
    {
        Write-Output($text)
        exit -1
    }
}

function _govet()
{
    logmsg("Running 'go vet' ...")

    $text = & go vet $PACKAGES
    fastfail("failed to run go vet $_")
    
    if ($text.length -ne 0)
    {
        Write-Output($text)
        exit -1
    }
}

function _unittest_with_coverage
{
    logmsg "Running unit tests ..."
    $covermode = "atomic"

    New-Item -ItemType Directory -path ($BUILD_DIR + "/test-reports") -force | Out-Null
    New-Item -ItemType Directory -path ($BUILD_DIR + "/coverage-reports") -force | Out-Null

    $profilecovfile = ($BUILD_DIR + "/coverage-reports/profile.cov")
    $coveragexmlfile = ($BUILD_DIR + "/coverage-reports/coverage.xml")

    ("mode: " + $covermode) | Out-File -encoding ascii -FilePath $profilecovfile

    foreach ($import_path in $PACKAGES) 
    { 
        $package = Split-Path -Leaf $import_path
        $covfile = ($BUILD_DIR + "/coverage-reports/profile_" + $package + ".cov")
        $repfile = ($BUILD_DIR + "/test-reports/" + $package + "-report.xml")
        Write-Output "Running tests for $import_path"

        $allargs = @( "test", "-v", "-race", "-covermode=$covermode", "-coverprofile=$covfile", $import_path )
        $testoutput = &'go' $allargs
        $testoutput | Out-Default
        fastfail("Unittests failed!")
        $testoutput | &go-junit-report.exe | Out-File -encoding ascii $repfile
    }

    Get-ChildItem -File ($BUILD_DIR + "/coverage-reports/") -Filter "profile_*.cov" |
    foreach-object  {
        Write-Output ("Processing coverage report for $_.FullName")
        Get-Content $_.FullName | select -Skip 1 | Out-File -encoding ascii -Append -FilePath $profilecovfile
        Remove-Item -Path $_.FullName
    }

    &go tool cover -func $profilecovfile
    fastfail("Failed to display coverage profile info for functions")

    &gocov convert $profilecovfile| &gocov-xml | Out-File -Encoding ascii -FilePath $coveragexmlfile
    fastfail ("Failed to convert coverage information to $coveragexmlfile")
}

function _getdeps()
{
    logmsg("Getting dependencies required for testing")

    & go get -u github.com/kardianos/govendor
    fastfail("failed to 'go get -u github.com/kardianos/govendor'")

    & govendor init
    fastfail("failed to 'govendor init'")

    & govendor fetch gopkg.in/square/go-jose.v2
    fastfail("Failed to govendor fetch gopkg.in/square/go-jose.v2")

    & govendor fetch gopkg.in/square/go-jose.v2/jwt
    fastfail("Failed to govendor fetch gopkg.in/square/go-jose.v2/jwt")

    & govendor fetch github.com/pkg/errors
    fastfail("failed to 'govendor fetch github.com/pkg/errors'")

    & go get -u github.com/jstemmer/go-junit-report
    fastfail("failed to 'go get -u github.com/jstemmer/go-junit-report'")

    & go get -u github.com/smartystreets/goconvey
    fastfail("failed to 'go get -u github.com/smartystreets/goconvey'")

    & go get -u golang.org/x/tools/cmd/cover
    fastfail("failed to 'go get -u golang.org/x/tools/cmd/cover'")

    & go get -u github.com/axw/gocov/...
    fastfail("failed to 'go get -u github.com/axw/gocov/...'")

    & go get -u github.com/AlekSi/gocov-xml
    fastfail("failed to 'go get -u github.com/AlekSi/gocov-xml'")
}

function main
{
    $ErrorActionPreference = "Stop"

    Remove-Item .\build -Force -Recurse -ErrorAction Ignore

    _getdeps

    logmsg('Generating $SUBDIR list...')
    $imports = & go list "-f" "{{.Dir}}" "./..."
    fastfail("failed to run go list...")

    $SUBDIRS = $imports

    logmsg('Generating $PACKAGES list...')

    $imports = & go list  "./..."
    fastfail("failed to run go list...")

    $PACKAGES = $imports

    $SOURCE_DIR = & git rev-parse --show-toplevel
    fastfail("Failed to find top-level source directory")

    $BUILD_DIR = $SOURCE_DIR + "/build"

    _gofmt
    _goimports
    _golint
    _govet
    _unittest_with_coverage
}


main
