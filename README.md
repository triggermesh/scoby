![TriggerMesh Logo](docs/assets/images/triggermesh-logo.png)

![CodeQL](https://github.com/triggermesh/scoby/actions/workflows/codeql.yaml/badge.svg?branch=main)
![Static](https://github.com/triggermesh/scoby/actions/workflows/static.yaml/badge.svg?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/triggermesh/scoby)](https://goreportcard.com/report/github.com/triggermesh/scoby)
[![Release](https://img.shields.io/github/v/release/triggermesh/scoby?label=release)](https://github.com/triggermesh/scoby/releases)
[![Slack](https://img.shields.io/badge/Slack-Join%20chat-4a154b?style=flat&logo=slack)](https://join.slack.com/t/triggermesh-community/shared_invite/zt-1kngevosm-MY7kqn9h6bT08hWh8PeltA)

Generic Kubernetes controller for simple workloads.

![scoby](docs/assets/images/harrison-kugler-kombucha.jpg)
> photo by [Harrison Kugler](https://unsplash.com/@harrisonkugler?utm_source=unsplash&utm_medium=referral&utm_content=creditCopyText)

## Description

Scoby is a controller that creates controllers dynamically :infinity:, and makes it easy to manage your application instances as Kubernetes objects.

In a nutshell, Scoby is the shortest path between your application's container image and Kubernetes end users.

![scoby user overview](docs/assets/images/scoby-user-overview.png)

Given a container image containinng an application, a Kubernetes CRD that defines the application spec, and an Scoby registration that configures rendering, end users will be able to manage instances of your application at Kubernetes.

## Install

To install Scoby at a Kubernetes cluster apply manifests for both CRDs and Controller:

```console
# Install Scoby CRDs
kubectl apply -f https://github.com/triggermesh/scoby/releases/latest/download/scoby-crds.yaml

# Install Scoby Controller
kubectl apply -f https://github.com/triggermesh/scoby/releases/latest/download/scoby.yaml
```

Refer to [releases](https://github.com/triggermesh/scoby/releases) for further information.

### Development Version

Development version can be installed using [ko](https://github.com/ko-build/ko)

```console
ko apply -f ./config
```

## Primer

In this primer we are setting up the minimal registration to start using Scoby and allow Kubernetes users to create instances of your example application.

There are 4 requirements to configure an application at Scoby:

- Container image.
- Custom Resource Definition file (CRD).
- ClusterRoles on the CRD to allow Scoby to manage it.
- CRD Registration for Scoby.

### Kuard Image

We need a container image that Scoby can use, and that will receive parameters via environment variables.

In this primer (and at some other samples) we are using the [Kuard application]((https://github.com/kubernetes-up-and-running/kuard)) image which was created by the authors of Kubernetes Up and Ready book, and runs a web server that enables us to inspect the environment variables configured.

The image is available at `gcr.io/kuar-demo/kuard-amd64:blue`

### Kuard CRD

Kubernetes can serve third party objects using [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/).

Scoby users need to create a CRD schema containing the fields that Kubernetes end users will provide to create new instances of the chosen container image.

By default each CRD schema field will generate an environment variable at the container, but rendering can be highly customized.

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kuards.extensions.triggermesh.io
spec:
  group: extensions.triggermesh.io
  scope: Namespaced
  names:
    plural: kuards
    singular: kuard
    kind: Kuard
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                # Primer Scoby sample variable
                primer:
                  type: string
```

The CRD above defines a `kuard` resource with an schema that lets users define a value for `spec.primer` as a string. It lacks validations, complex structures and status but is still good to get to know Scoby.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/00.primer/01.kuard-crd.yaml
```

### Kuard ClusterRoles

### CRD Registration

- Create the CRDRegistration: the registration informs Scoby about how the CRD elements are transformed into environment variables, what image to use, and what type of workload should be created.

A registration could look as simple as this:

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: myapp
spec:
  crd: myapp.myorganization.io
  workload:
    fromImage:
      repo: myorganization/myapp:v1
```

Once the registration is created Scoby will spin up a controller that reconciles `myapp` resource instances created by end users and creates a workload for them. Scoby takes care of rendering the workload according to the registration data, and maintain the status according to the registered CRD.

![scoby end user overview](docs/assets/images/scoby-end-user-overview.png)

## Usage

Any valid CRD that is valid for Kubernetes will work with Scoby. If the CRD contains the status subresource and it adheres to the [recommended structure](docs/status.md), Scoby will fill it upon reconciliation.

A `CRDRegistration` object contains:

- `formFactor` determines the objects that will be created for each instance of the Custom Resource created.
- `parameterConfiguration` hints how to parse elements in the Custom Resource to convert them to environment variables that will be consumed by the container.

This example registration creates a controller for the user provided CRD `my-example.existing.crd` and image `my-repo/my-example:v1.0.0`:

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: my-example-registration
spec:
  crd: my-example.existing.crd
  workload:
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
    fromImage:
      repo: my-repo/my-example:v1.0.0
    parameterConfiguration:
      customize:
      - path: spec.account.name
        render:
          key: MY_EXAMPLE_AUTH_USER
      - path: spec.account.passwordSecret
        render:
          key: MY_EXAMPLE_AUTH_PASSWORD
          valueFromSecret:
            name: spec.account.passwordSecret.name
            key: spec.account.passwordSecret.key
```

- At the `spec.workload.formFactor` section it is instructed to create a deployment and connect it with a service that will expose port 80 externally and redirect requests to 8080 at the container.
- The image for the deployment is referenced at `.spec.workload.fromImage.repo`
- Parameters for the deployment's container will be customized following `.spec.workload.parameterConfiguration` rules.
  - If Custom Resources created by users contain a `.spec.account.name` element, an environment variable named `MY_EXAMPLE_AUTH_USER` will be created using the element's value.
  - If Custom Resources created by users contain a `.spec.account.passwordSecret` element, an environment variable named `MY_EXAMPLE_AUTH_PASSWORD` will be created using a Kubernetes secret reference as value.

For further information

- :computer_mouse: Start using Scoby with the [getting started guide](docs/getting-started/README.md).
- :bookmark_tabs: Learn more about registration at the [registration documentation](docs/registration.md).
