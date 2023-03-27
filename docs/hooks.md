# (NOT IMPLEMENTED) Controller Hooks

Scoby reconciliation process is limited to the form factor and environment variable generation capabilities due to its declarative nature.

For cases where further control is needed hooks can be used at reconciliation cycles. Hooks are user provided services that are called at each reconciliation cycle, and whose reply can shape the produced workload and the object's status.

## Registering the Hook

A Hook is defined within a registration, and points to either an URI or a reference to an addressable object or a service. When a reconciliation cycle occurs Scoby identifies if the objject is being deleted or not, and sends a reconciliation request to the Hook address that includes:

- a reference to the object (namespace, name, apiVersion, kind)
- the operation being executed (reconcile or finalize)

To register a hook, use the `spec.hook` element in the registration.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
  hook:
    address:
      # Either an URI
      uri: http://my-hook-service
      # Or an URL
      ref:
        apiVersion: v1
        kind: Service
        name: my-service

    # ISO 8601 duration
    timeout: PT10S

    # Initialization and finalization are the 2 operations available, which
    # are enabled by default and will use the latest version if not explicitly
    # indicated.
    initialization:
      enabled: true
      version: 1
    finalization:
      enabled: true
      version: 1

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

Request contains the object reference and operation, which can be `reconcile` or `finalize`.

```json
{
    "object": {
        "apiVersion": "v1alpha1",
        "kind": "Kuard",
        "namespace": "my-namespace",
        "name": "my-kuard"
    },
    "operation": "reconcile"
}
```

Response contains the status conditions and environment variables.
If the status conditions are not `True` Scoby will update the object's status and stop the reconciliation.

```json
{
    "status": {
        "conditions":
        [
            {
                "type": "KuardReady",
                "status": "True"
            }
        ]
    },
    "envVars":
    [
        {
            "key": "MY_ENV",
            "value": "value1"
        }
    ]
}
```
