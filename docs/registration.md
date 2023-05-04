# Registration

Registrations are objects that informs the Scoby controller to manage custom objects.

Right now only the `CRDRegistration` object is available, but new registration could be created pursuing a better user experience.

Registration CRD has 3 elements under its spec:

- `spec.crd` be provided and should point to an existing CRD whose instances will be watched by the controller.
- `spec.workload` must be provided and inform of the container image to be used for each instance of the registered object and the form factor it should create.
- `spec.hook` is an optional element that allows the reconciliation process to call an external service to provide extended functionality to Scoby.

## CRD

Any CRD is subject to be controlled by Scoby, although it is a recommended practice to get familiar with Scoby registration and to keep it simple, provide parameter transformation from Kubernetes objects to environment variables, and obtain meaningful statuses.

Scoby does not perform validation on user objects, registered CRDs should rely on  Kubernetes OpenAPI validation features.

When designing your CRD make sure that user provided data lives under the `.spec` element, all subelements will be considered for being transformed into environment variables.

It is highly recommended to follow `.status` [Kubernetes status structure](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status) conventions to make sure Scoby fills provides accurate status. Scoby can also fill information for exposed URL, observed generation and annotations as [described here](./status.md).

Scoby needs to be granted permissions to manage the CRD. This can be done creating a `ClusterRole` that contains the label `scoby.triggermesh.io/crdregistration: true`, and the permissions that it needs. You can use this template:

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: crd-registrations-scoby-kuard
  labels:
    scoby.triggermesh.io/crdregistration: "true"
    app.kubernetes.io/name: scoby
# Do not use this role directly. These rules will be added to the "crd-registrations-scoby" role.
rules:
- apiGroups:
  - <REPLACE-WITH-APIGROUP>
  resources:
  - <REPLACE-WITH-RESOURCE>
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - <REPLACE-WITH-APIGROUP>
  resources:
  - <REPLACE-WITH-RESOURCE>/status
  verbs:
  - update
```

## Workload

Workload is informed using `.spec.workload` and contains rendering customization for reconciling end user Kubernetes instances, and executing tasks to obtain generated Kubernetes objects acording to the instance's spec.

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

Customization for generated parameters is possible through the `spec.workload.parameterConfiguration`

### Global Customization

- [x] Define global prefix for all generated environment variables.

```yaml
    parameterConfiguration:
      global:
        defaultPrefix: FOO_
```

### Add New Parameters

- [x] Create new parameter with literal value

```yaml
    parameterConfiguration:
      addEnvs:
      - name: FOO_NEW_VAR
        value: 42
```

- [x] Create new parameter with downward api value

```yaml
    parameterConfiguration:
      addEnvs:
      - name: FOO_NEW_VAR
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
```

- [x] Create new parameter with secret

```yaml
    parameterConfiguration:
      add:
      - name: FOO_MY_CREDS
        valueFrom:
          secretKeyRef:
            name: mycreds
            key: token
```

- [x] Create new parameter with configmap

```yaml
    parameterConfiguration:
      add:
      - name: FOO_MY_BACKGROUND
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
          name: FOO_BAR
```

- [x] Change value to literal.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.bar
        render:
          value: hello scoby
```

- [x] Generate secret parameter from element.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.credentials
        render:
          name: FOO_CREDENTIALS
          valueFromSecret:
            name: spec.credentials.name
            key: spec.preferences.key
```

- [x] Generate configmap parameter from element.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.preferences
        render:
          valueFromConfigmap:
            name: spec.preferences.name
            key: spec.preferences.key
```

- [x] Function: resolve object to internal URL

```yaml
    parameterConfiguration:
      customize:
      - path: spec.destination
        render:
          name: K_SINK
          valueFromBuiltInFunc:
            name: resolveAddress
```

## Workload Status

- [x] Use parameter value for status.

```yaml
    parameterConfiguration:
      customize:
      - path: spec.destination
        render:
          name: K_SINK
          valueFromBuiltInFunc:
            name: resolveAddress
    statusConfiguration:
      addElements:
      - path: status.sinkUri
        render:
          valueFromParameter:
            path: spec.destination
```

## Examples

The [getting started guide](getting-started/README.md) drives you through the [examples found at the Scoby repository](https://github.com/triggermesh/scoby/tree/main/docs/samples/01.kuard).
