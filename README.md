# :hammer: dig [![GoDoc][doc-img]][doc] [![GitHub release][release-img]][release] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov] [![Go Report Card][report-card-img]][report-card]

A reflection based dependency injection toolkit for Go.

### Good for:

* Powering an application framework, e.g. [Fx](https://github.com/uber-go/fx).
* Resolving the object graph during process startup.

### Bad for:

* Using in place of an application framework, e.g. [Fx](https://github.com/uber-go/fx).
* Resolving dependencies after the process has already started.
* Exposing to user-land code as a [Service Locator](https://martinfowler.com/articles/injection.html#UsingAServiceLocator).

## Installation

We recommend locking to [SemVer](http://semver.org/) range `^1` using [Glide](https://github.com/Masterminds/glide):

```
glide get 'go.uber.org/dig#^1'
```

## Stability

This library is `v1` and follows [SemVer](http://semver.org/) strictly.

No breaking changes will be made to exported APIs before `v2.0.0`.

[doc-img]: http://img.shields.io/badge/GoDoc-Reference-blue.svg
[doc]: https://godoc.org/go.uber.org/dig

[release-img]: https://img.shields.io/github/release/uber-go/dig.svg
[release]: https://github.com/uber-go/dig/releases

[ci-img]: https://img.shields.io/travis/uber-go/dig/master.svg
[ci]: https://travis-ci.org/uber-go/dig/branches

[cov-img]: https://codecov.io/gh/uber-go/dig/branch/master/graph/badge.svg
[cov]: https://codecov.io/gh/uber-go/dig/branch/master

[report-card-img]: https://goreportcard.com/badge/github.com/uber-go/dig
[report-card]: https://goreportcard.com/report/github.com/uber-go/dig
