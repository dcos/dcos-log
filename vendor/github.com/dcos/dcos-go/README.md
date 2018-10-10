# dcos-go - DC/OS golang shared packages
[![Build Status](https://jenkins.mesosphere.com/service/jenkins/buildStatus/icon?job=public-dcos-go/public-dcos-go-master)](https://jenkins.mesosphere.com/service/jenkins/job/public-dcos-go/job/public-dcos-go-master/)
[![GoDoc](https://godoc.org/github.com/dcos/dcos-go?status.svg)](https://godoc.org/github.com/dcos/dcos-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/dcos/dcos-go)](https://goreportcard.com/report/github.com/dcos/dcos-go)

This repository exists to collect commonly shared Go code within Mesosphere. It can include code that is specific to DC/OS and can also include code that is more utilitarian in nature.

## Contributing

Everyone is welcome to submit Pull Requests.  

If you find yourself writing the same code more than once in separate projects, or have seen the same code in more than one project, it may be a good candidate for inclusion into this repository.  Be sure to ask on the [mailing list](https://groups.google.com/a/mesosphere.io/forum/#!forum/go) if you are unsure before contributing.

Pull requests should adhere to the following guidelines:

* Reviewer first: create your changeset with care for the reviewer. 
* The changeset should have a good description. 
* Each commit in the changeset should have a description as well as be focused on one thing.
* Code must be tested
* Code should be written with care to allow for non-breaking changes going forward
* If introducing a new package, the package should have a `doc.go` file which describes the purpose of the package.
* Try not to introduce external dependencies unless necessary.

## External Libraries

This project uses [dep](https://github.com/golang/dep) to manage external dependencies.  This tool vendors external libs into the `/vendor` directory.  There is one set of dependencies for all of the packages that live in this project.  If you need to make a change to one of the dependencies, at the current time it will need to be compatible with all of the packages.  A passing CI build will ensure this.

## Packages In This Library
- [jwt/transport](/jwt/transport/README.md) : JWT token support.
- [store](/store/README.md) : In-Memory key/value store.
- [zkstore](/zkstore/README.md): ZK-based blob storage.
- [dcos/nodeutil](/dcos/nodeutil/README.md) : Interact with DC/OS services and variables
- [elector](/elector/README.md): Leadership election.

Note that this package list is manually updated in this README.  There is some discussion about automating this process.  You can track the progress of this effort by following [this ticket](https://jira.mesosphere.com/browse/DCOS_OSS-1475).

## OSS Projects Using This Library
- [dcos-log](https://github.com/dcos/dcos-log)
- [dcos-metrics](https://github.com/dcos/dcos-metrics)

## License
Licensed under the [Apache License, Version 2.0](LICENSE).
