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

- Define global prefix for all generated environment variables.

```yaml
    parameterConfiguration:
      global:
        defaultPrefix: FOO_
```

The prefix will not be applied to parameters where an explicit name is provided for the environment variable.

### Add New Environment Variables

- Create new parameter with literal value

```yaml
    parameterConfiguration:
      add:
        toEnv:
        - name: FOO_NEW_VAR
          value: 42
```

- Create new parameter with downward api value

```yaml
    parameterConfiguration:
      add:
        toEnv:
        - name: FOO_NEW_VAR
          valueFrom:
            field:
              fieldPath: metadata.name
```

- Create new parameter with secret

```yaml
    parameterConfiguration:
      add:
        toEnv:
        - name: FOO_MY_CREDS
          valueFrom:
            secret:
              name: mycreds
              key: token
```

- Create new parameter with configmap

```yaml
    parameterConfiguration:
      add:
        toEnv:
        - name: FOO_MY_BACKGROUND
          valueFrom:
            configMap:
              name: mypreferences
              key: background
```

- Create new parameter with configmap values from the Scoby controller's namespace.

```yaml
    parameterConfiguration:
      add:
        toEnv:
        - name: FOO_MY_LOGGING
          valueFromControllerConfigMap:
            name: observability
            key: logging
```

Permissions for reading ConfigMaps at the controller's namespace must be granted.
The referenced ConfigMap and key must exist.

### Customize Parameters From Spec

The default behavior is to create parameters from each element under `spec` found at incoming objects (arrays will create just one element that includes any sub-elements). When a parameter customization is found, the default parameter generation for that element or any sub-element to the one indicated, will be skipped.

- Avoid producing parameter.

```yaml
    parameterConfiguration:
      fromSpec:
        skip:
        - path: spec.bar
```

- Change key for generated param.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.bar
          name: FOO_BAR
```

- Use default literal value to element when not informed.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.bar
          default:
            value: hello scoby
```

- Use default configmap value to element when not informed.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.location
          default:
            configMap:
              name: config
              key: country
```

- Use default secret value to element when not informed.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.username
          default:
            secret:
              name: creds
              key: user
```

- Generate configmap parameter from `spec` element.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.preferences
          valueFrom:
            configMapPath:
              name: spec.preferences.name
              key: spec.preferences.key
```

- Generate secret parameter from `spec` element.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.credentials
          name: FOO_CREDENTIALS
          valueFrom:
            secretPath:
              name: spec.credentials.name
              key: spec.credentials.key
```

- Function: resolve [addressable object](https://knative.dev/docs/eventing/sinks/) to internal URL

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.destination
          name: K_SINK
          valueFrom:
            builtInFunc:
              name: resolveAddress
```

Addressable objects might be Kubernetes services or references to objects that inform a URL address at `status.address.url`. References at the object instance must follow this structure.

```yaml
    ref:
      apiVersion: <APIVERSION REFERENCE>
      kind: <KIND REFERENCE>
      name: <OBJECT NAME REFERENCE>
    uri: <COMPLETE OR PARTIAL URI>
```

### Generate Volumes From Spec

Secrets and ConfigMaps can be mounted as a volume inside the workload. The registration needs a name for the volume, the file to mount inside the container and a reference to the Secret or ConfigMap.

- [x] Function: resolve object to internal URL

```yaml
    parameterConfiguration:
      fromSpec:
        toVolume:
        - path: spec.userList
          name: userfile
          mountPath: /opt/user.lst
          mountFrom:
            configMap:
              name: spec.userList.name
              key: spec.userList.key
```

## Workload Status

- [x] Use parameter value for status.

```yaml
    parameterConfiguration:
      fromSpec:
        toEnv:
        - path: spec.destination
          name: K_SINK
          valueFrom:
            builtInFunc:
              name: resolveAddress
    statusConfiguration:
      add:
      - path: status.sinkUri
        valueFrom:
          path: spec.destination
```

## Examples

The [Scoby tutorial](../tutorial.md) drives you through the [examples found at the Scoby repository](https://github.com/triggermesh/scoby/tree/main/docs/samples/01.kuard).
