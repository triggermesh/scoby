# Registration

Registrations are the objects that drive Scoby controllers to manage users CRDs.

Right now only the `CRDRegistration` object is available, but new registration could be created pursuing a better user experience.

(New registrations should expose to users a custom YAML and internally convert their data into `CRDRegistration`.)

## Registration CRD

Registration CRD needs information about:

- CRD: this must be provided by users, and should contain the user data under the `.spec` element and a standard `.status` [Kubernetes structure](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).
- Workload: must inform the container image to be used for each instance of the registered object and form factor it should create, which by default is a regular `Deployment`.

## Workload FormFactor

The workload form factor options let users generate `Deployment` or Knative `Service`, each of them with a set of parameters:

For a `Deployment` the parameters are the number of replicas and if a Kubernetes `Service` should be included.

```yaml
spec:
  workload:
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
```

For a Knative `Service` the scaling parameters and visibility can be informed.

```yaml
spec:
  workload:
    formFactor:
      knativeService:
        minScale: 0
        maxScale: 5
        visibility: cluster-local
```

When no `spec.workload.formFactor` element is informed, `Deployment` is defaulted.

## Workload Parameter Configuration

Scoby uses instances of registered CRDs to create the workload, passing the instance's data via environment variables. Default instance data parsing is:

- All elements under `.spec` are considered data to be available at the workload.
- Environment variable names will be generated after each element path, joining them with underscores and using capital letters.
- Values for arrays of primitive elements will be serialized as a single string consisting of comma separated values.
- Values for arrays of complex elements (that contain sub-elements) will be serialized as a single string consisting of a JSON marshalled representation of the inner elements.

TODO add example

Customization for generated parameters is possible through the `spec.workload.parameterConfiguration`

Customization is being worked on, A checkbox indicates if the feature is available at main branch.

### Global Customization

- [ ] Define global prefix for all generated environment variables.

```yaml
    parameterConfiguration:
      global:
        defaultPrefix: FOO_
```

### Add New Parameters

- [ ] Create new parameter with literal value

```yaml
    parameterConfiguration:
      add:
      - key: FOO_NEW_VAR
        value: 42
```

- [ ] Create new parameter with downward api value

```yaml
    parameterConfiguration:
      add:
      - key: FOO_NEW_VAR
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
```

- [ ] Create new parameter with secret

```yaml
    parameterConfiguration:
      add:
      - key: FOO_MY_CREDS
        valueFrom:
            secretKeyRef:
              name: mycreds
              key: token
```

- [ ] Create new parameter with configmap

```yaml
    parameterConfiguration:
      add:
      - key: FOO_MY_BACKGROUND
        valueFrom:
            configMapKeyRef:
              name: mypreferences
              key: background
```

### Customize Parameters From Spec

The default behavior is to create parameters from each spec element (arrays will create just one element that includes any sub-elements). When a parameter customization is found, the default parameter generation for that element or any sub-element to the one indicated, will be skipped.

- [x] Avoid producing parameter.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.bar
        render:
          skip: true
```

- [x] Change key for generated param. Can be combined.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.bar
        render:
          key: FOO_BAR
```

- [ ] Change value to literal.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.bar
        render:
          value: hello scoby
```

- [ ] Generate secret parameter from element.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.credentials
        render:
          key: FOO_CREDENTIALS
          valueFromSecret:
            name: spec.credentials.name
            key: spec.preferences.key
```

- [ ] Generate configmap parameter from element.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.preferences
        render:
          valueFromConfigmap:
            name: spec.preferences.name
            key: spec.preferences.key
```

- [ ] Function: resolve object to internal URL

```yaml
    parameterConfiguration:
      customize:
      - path: spec.destination
        render:
          key: K_SINK
          valueFromBuiltInFunc:
            name: resolveAddress
```

## Workload Status

TODO, this is a draft.

- [ ] Add status annotation.
- [ ] Use parameter value for annotation.
- [ ] Use parameter value for status.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.destination
        render:
          valueFromBuiltInFunc:
            name: resolveAddress
    statusConfiguration:
      - path: status.uri
        render:
          valueFromParameter:
            path: spec.destination
```

## Examples

TODO, see [kuard example](https://github.com/triggermesh/scoby/tree/main/docs/samples/01.kuard) while the examples section is worked on.

### Pre-requisite

All examples require Kuard CRD to be installed

```console
kubectl apply -f https://github.com/triggermesh/scoby/tree/main/docs/samples/01.kuard/01.kuard-crd.yaml
```

Kuard is an application created by the authors of Kubernetes Up and Ready book that is able to render a list of its environment variables. Our registration uses the Kuard image and a list of elements at the spec to demonstrate how to render them as environment variables in different ways.

### Example with URL resolving

A registered object is able to resolve a kubernetes object into a URL and use the result as an environment variable.

// TODO
