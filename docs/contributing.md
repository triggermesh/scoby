# Contributing to TriggerMesh

Welcome, contributors! :wave:

Thank you for taking the time to go through this document, which suggests a few guidelines for contributing to the
TriggerMesh open source platform.

We define _contributions_ as:

- Bug reports
- Feature and enhancement requests
- Code submissions
- Any participation in discussions within the TriggerMesh community

## Contents

- [Contributing to TriggerMesh](#contributing-to-triggermesh)
  - [Contents](#contents)
  - [Code of Conduct](#code-of-conduct)
  - [Submitting Contributions](#submitting-contributions)
    - [Reporting Bugs](#reporting-bugs)
      - [Issue Description](#issue-description)
      - [Code Styling](#code-styling)
    - [Requesting Features and Enhancements](#requesting-features-and-enhancements)
    - [Submitting Code Changes](#submitting-code-changes)
  - [Development Guidelines](#development-guidelines)
    - [Prerequisites](#prerequisites)
      - [Go Toolchain](#go-toolchain)
      - [Kubernetes](#kubernetes)
      - [ko](#ko)
    - [Running the Controller](#running-the-controller)
      - [Controller Configuration](#controller-configuration)
      - [Running Locally](#running-locally)
      - [Running With KO](#running-with-ko)
  - [License Headers](#license-headers)
  - [Logs](#logs)
    - [Verbosity](#verbosity)

## Code of Conduct

Although this project is not part of the [CNCF][cncf], we abide by its [Code of Conduct][cncf-coc], and expect all
contributors to uphold this code. Please report unacceptable behavior to <info@triggermesh.com>.

## Submitting Contributions

The guidelines below aim at ensuring that maintainers can understand submissions as quickly and effortlessly as
possible, whether these are questions, issue reports, feature requests, or code contributions. The golden rule is: the
clearer the information, the faster the resolution :rocket:.

### Reporting Bugs

Bugs, or any kind of issue you encounter with the TriggerMesh platform, can be reported using [GitHub Issues][gh-issue].

Before opening a new issue, kindly [search for a few keywords][gh-search] related to the problem you encountered, just
to ensure a similar report hasn't already been submitted. Didn't find anything relevant? Great! :+1: Let's create that
issue.

:information_source: _If you suspect or discover a security vulnerability in the TriggerMesh software, please do not
disclose it publicly via a GitHub issue. Instead, please report it to <info@triggermesh.com> so that maintainers can
address it within the shortest possible delay._ :lock::hourglass:

#### Issue Description

A good bug report starts with a clear and descriptive title. Avoid overly generic titles such as "bug in component X",
or raw outputs from error logs. Indicate _what_ is failing and, if known, under _what circumstances_ it is failing (e.g.
"Component X panics when environment variable Y is not set").

Although there is no enforced template for submitting issues, we do recommend including the following information:

- A detailed description, in plain English, of the behaviour you are observing, and what you expected instead.
- The release version of Scoby the problem can be observed with (or software revision if the software
  was built from source).
- The component that is affected (registration, rendering, hook, etc.)
- A configuration snippet that can be used to reproduce the issue.
- If some preliminary setup of a third-party service was performed, please describe those steps.
- The error messages you are seeing, if any.
  - :bulb: Most errors are reported via Kubernetes API events and object statuses. Both can be obtained using the
    [kubectl describe][k-describe] command.

Remember, anything that allows maintainers to reproduce the problem from the _initial_ issue description is another day
saved going back and forth to the issue's comments to ask for additional information, leading to _your_ issue being
solved faster! :raised_hands:

#### Code Styling

To ensure the indentation of command outputs and the highlighting of code snippets are preserved inside the text of
GitHub issues, we recommend wrapping them inside [Fenced Code Blocks][gh-fenced] using the triple backticks notation
(` ``` `).

Examples:

<table>

<thead>
<tr>
<th>Raw Markdown</th>
<th>Rendered code block</th>
</tr>
</thead>

<tbody>
<tr>
<td>

````
```
[2021/12/01 15:43:04] Some log output
```
````

</td>
<td>

```
[2021/12/01 15:43:04] Some log output
```

</td>
</tr>

<tr>
<!–– NOTE: empty row to prevent alternate row highlight a.k.a. "zebra striping" -->
</tr>

<tr>
<td>

````
```yaml
# My Kubernetes manifest
apiVersion: triggermesh.io/v1
```
````

</td>
<td>

```yaml
# My Kubernetes manifest
apiVersion: triggermesh.io/v1
```

</td>
</tr>
</tbody>

</table>

### Requesting Features and Enhancements

Features and enhancements can be reported using [GitHub Issues][gh-issue].

Similarly to [bug reports](#reporting-bugs), we kindly ask you to [search for a few keywords][gh-search] related to your
suggestion before opening a new issue, just to ensure a similar request hasn't already been submitted. In case that
search doesn't yield any relevant result, let's go ahead and create that issue. :memo:

Provide a detailed description, in plain English, of the result you are expecting by submitting your request:

- If you are asking for an enhancement to an existing component, be specific about which component. If possible, include
  links to any external resources that may help maintainers get a clearer understanding of the desired outcome.
- If you are asking for a new feature, please clarify the nature of that feature. Examples include:
  - A new integration with a third-party service.
  - A new data processor.
  - A new authentication method.
  - ...

The clearer the request, the easier it is for maintainer to discuss a potential design and implementation!
:raised_hands:

### Submitting Code Changes

Code submissions can be proposed using [GitHub Pull Requests][gh-pr].

Whenever a pull request is opened, and every time it is updated by pushing new commits, the CI pipeline performs some
static code analysis on the submitted code revision to ensure that certain [code styles][ci-linters] are respected. All
status checks must be passing for a pull request to be considered by maintainers. :heavy_check_mark:

Small, non-breaking changes, can be submitted spontaneously without prior discussion with maintainers, providing that
they include a clear justification of their potential relevance to the project.

Larger changes such as new features, or changes which impact the current behaviour of certain TriggerMesh components,
should be socialized and discussed with maintainers in a [GitHub Issue][gh-issue]. Nobody likes seeing a submission getting rejected because it was not aligned with
the project's goals or standards! :disappointed:

If you read that far and are feeling ready to submit your first code contribution, congratulations! :heart: Read on, the
following section about [Development Guidelines](#development-guidelines) explains what our standard development
environment looks like.

## Development Guidelines

### Prerequisites

- Go toolchain
- Kubernetes
- ko

#### Go Toolchain

TriggerMesh is written in [Go][go].

The Go toolchain is required in order to be able to compile the TriggerMesh code and run its automated tests.

The currently recommended version can be found inside the [`go.mod`](go.mod) file, based on the `go` directive.

#### Kubernetes

TriggerMesh runs on top of the [Kubernetes][k8s] platform.

Any certified Kubernetes distribution can run TriggerMesh, whether it is running locally (e.g. using [`kind`][k8s-kind],
inside a virtual machine, ...) or remotely (e.g. inside a cloud provider, in your own datacenter, ...).

The currently recommended version can be found inside the [`go.mod`](go.mod) file, based on the version of the
`k8s.io/api` module dependency.

#### ko

[`ko`][ko] is a tool which allows developers to package Go projects as container images and deploy them to Kubernetes in
a single command, all of this without requiring Docker to be installed.

TriggerMesh relies on `ko` extensively, both for development purposes and for its own releases.

We recommend using version `v0.9.0` or greater.

### Running the Controller

#### Controller Configuration

The Scoby controller uses [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) and follows its configuration approach.

#### Running Locally

It is possible to run the TriggerMesh controller (`cmd/scoby-controller/main.go`) locally, and let it operate against
_any_ Kubernetes cluster, whether this cluster is running locally (e.g. inside a virtual machine) or remotely (e.g.
inside a cloud provider).

:warning: **Before running the controller in your local environment, make sure no other instance of the TriggerMesh
controller is currently running inside the target cluster! This could prevent your local instance from performing any
work due to the leader-election mechanism, or worse, result in multiple controllers performing conflicting changes
simultaneously to the same objects.**

Providing that _(1)_ the local environment is configured with a valid [kubeconfig][k8s-kubecfg] and _(2)_ the
aforementioned [mandatory environment variables](#configuration-read-from-the-environment) are exported, running the
controller locally from the current development branch is as simple as executing:

To setup a development environment you need to create the `CRDRegistration` object first, then run the controller:

```console
# Apply registration
kubectl apply -f config/300-crdregistration.yaml

# Run controller
go run cmd/scoby-controller/main.go --zap-log-level 5
```

#### Running With KO

Development version can be installed at the configured cluster using [ko](https://github.com/ko-build/ko)

```console
ko apply -f ./config
```

## License Headers

License headers must be written using SPDX identifier.

```go
// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0
```

Use [addlicense](https://github.com/google/addlicense) to automatically add the header to all go files.

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```

## Logs

The controller rely on `controller-runtime` which uses the [logr](https://github.com/go-logr/logr) wrapper over [zap](https://github.com/uber-go/zap) logger.

- To enable development configuration use `--zap-devel`.
- Verbosity can be controlled by using `--zap-log-level <level>`.

### Verbosity

Scoby uses [logr](https://github.com/go-logr/logr) to write logs. When debugging we will use verbosity levels `0,1,5,10` in this way:

| V     | Description  |
|---    |---    |
| 0     | Always shown, equals to info level  |
| 1     | Equals to debug level |
| 2 - 5     | Use levels 2 - 5 for chatty debug logs |
| 6 - 10    | Use leves for spammy debug logs (eg. inside loops) |

[cncf]: https://www.cncf.io/
[cncf-coc]: https://github.com/cncf/foundation/blob/master/code-of-conduct.md

[gh-issue]: https://github.com/triggermesh/scoby/issues
[gh-search]: https://github.com/triggermesh/scoby/issues?q=
[gh-fenced]: https://docs.github.com/en/github/writing-on-github/working-with-advanced-formatting/creating-and-highlighting-code-blocks
[gh-pr]: https://github.com/triggermesh/scoby/pulls

[go]: https://golang.org/

[ko]: https://github.com/google/ko

[k8s]: https://kubernetes.io/
[k8s-kind]: https://kind.sigs.k8s.io/
[k-describe]: https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#describe
[ci-linters]: https://golangci-lint.run/usage/linters/#enabled-by-default-linters
