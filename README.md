# :hammer: dig [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov] [![Report Card][report-card-img]][report-card]

A reflection based dependency injection toolkit for Go.

### Good for:

* Powering an application framework, e.g. dig powers [Fx](https://github.com/uber-go/fx).
* Resolving the object graph during process startup.

### Bad for:

* Using in place of an application framework, like [Fx](https://github.com/uber-go/fx).
* Resolving deps once the process has already started.
* Using as a [Service Locator](https://martinfowler.com/articles/injection.html#UsingAServiceLocator).

## Status

Almost stable: `v1.0.0-rc1`. Expect minor API changes and bug fixes. See [our changelog](CHANGELOG.md)

## Install

```
go get -u go.uber.org/dig
```

[doc]: https://godoc.org/go.uber.org/dig
[doc-img]: https://godoc.org/go.uber.org/dig?status.svg
[cov]: https://codecov.io/gh/uber-go/dig/branch/master
[cov-img]: https://codecov.io/gh/uber-go/dig/branch/master/graph/badge.svg
[ci]: https://travis-ci.org/uber-go/dig
[ci-img]: https://travis-ci.org/uber-go/dig.svg?branch=master
[report-card]: https://goreportcard.com/report/github.com/uber-go/dig
[report-card-img]: https://goreportcard.com/badge/github.com/uber-go/dig
