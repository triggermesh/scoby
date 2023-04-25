# Controller Hooks

Scoby reconciliation process is limited to the form factor and environment variable generation capabilities due to its declarative nature.

For cases where further control is needed hooks can be used at reconciliation cycles. Hooks are user provided services that are called at each reconciliation cycle, and whose reply can shape the produced workload and the object's status.

## Registering the Hook

A Hook is defined within a registration, and points to either an URI or a reference to an addressable object or a service. When a reconciliation cycle occurs Scoby identifies if the objject is being deleted or not, and sends a reconciliation request to the Hook address that includes:

- a reference to the object (namespace, name, apiVersion, kind)
- hook's capabilities, that is, what operations it supports

To register a hook, use the `spec.hook` element in the registration.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
  hook:
    # Hook API implemented version.
    version: 1

    address:
      # URI
      uri: http://my-hook-service
      # Object reference
      #
      # When informing the object and URI at the same time, URI will provide
      # scheme, path and port information while object will be used to identify
      # the host.
      ref:
        apiVersion: v1
        kind: Service
        name: my-service

    # ISO 8601 duration
    timeout: PT10S

    # Capabilities that the hook implement.
    #
    # "pre-reconcile" is called before Scoby executes the generated object rendering from the reconiler.
    # "post-reconcile" (Not implemented) is called at reconciliation after Scoby has rendered.
    # "finalization" is called when an object has been deleted.
    capabilities:
    - pre-reconcile
    - post-reconcile
    - finalize

  workload:
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
    fromImage:
      repo: gcr.io/kuar-demo/kuard-amd64:blue

    parameterConfiguration:
      addEnvsFromHook:
      # All environment variables received as a response will be used
      # as workload env vars.
      - '*'

    statusConfiguration:
      # Look for this condition at response from webhook
      conditionsFromHook:
      - type: KuardReady
```

The Hook is called at each reconciliation.

## Hook API

Hooks use JSON payloads at both request and response.

Request contains the object reference and phase, which can be `pre-reconcile`, `post-reconcile` or `finalize`.

### Phase: pre-reconcile

`pre-reconcile` phase request includes a reference to the object that is being reconciled.

```json
{
    "object": {
        "apiVersion": "v1alpha1",
        "kind": "Kuard",
        "namespace": "my-namespace",
        "name": "my-kuard"
    },
    "phase": "pre-reconcile"
}
```

The response from the hook might include information about:

- `error`: to be filled if a processing error occurs. `permanent` subfield might be added to specify if the error should trigger a new reconciliation. `halt` is used to indicate if the reconciliation logic should stop after this error.

```json
{
  "error": {
    "message": "some error",
    "permantent": "true|false",
    "halt": "true|false"
  },
  "patches": [
    {"main":"object"},
    {"service":"account"}
  ]
}
```

### Phase: post-reconcile

`post-reconcile` phase request includes a reference to the object that is being reconciled plus a list of the rendered objects that Scoby produces.

Note: `post-reconcile`  IS NOT IMPLEMENTED YET.

```json
{
    "object": {
        "apiVersion": "v1alpha1",
        "kind": "Kuard",
        "namespace": "my-namespace",
        "name": "my-kuard"
    },
    "rendered": [...],
    "phase": "post-reconcile"
}
```

### Phase: finalize

`finalize` phase request includes a reference to the object that is being reconciled.

```json
{
    "object": {
        "apiVersion": "v1alpha1",
        "kind": "Kuard",
        "namespace": "my-namespace",
        "name": "my-kuard"
    },
    "phase": "finalize"
}
```

The response from the hook might include information about:

- `error`: to be filled if a processing error occurs. `permanent` subfield might be added to specify if the error should trigger a new reconciliation. `halt` is used to indicate if the reconciliation logic should stop after this error.

```json
{
  "error": {
    "message": "some error",
    "permantent": "true|false",
    "halt": "true|false"
  },
}
```

When returning an error the `halt` element is used to determine if the finalizer should be removed from the object or not; `true` means that the finalizer would not be removed from the object.
