# Controller Hooks

Scoby reconciliation process is limited to the form factor and environment variable generation capabilities due to its declarative nature.

For cases where further control is needed hooks can be used at reconciliation cycles. Hooks are user provided services that are called at each reconciliation cycle, and whose reply can shape the produced workload and the object's status.

:warning: Scoby Hooks are experimental, and although we use them to create TriggerMesh components and will try to keep backsward compatibility and a reliable versioning policy, it is in early stages of development and might change in the near future.

## Registering the Hook

A Hook is defined within a registration, and points to either an URI or a reference to an addressable object or a service. Registrations that include hooks might contain these `spec` fields:

```yaml
spec:
  hook:
    # Hook API implemented version.
    version: 1

    address:
      # URI/Object reference
      uri: <HOOK URI>
      ref:
        apiVersion: <HOOK OBJECT API VERSION>
        kind: <HOOK OBJECT KIND>
        name: <HOOK OBJECT NAME>

    # Optional HTTP timeout
    timeout: <ISO 8601 DURATION>

    # Array of Capabilities that the hook implement.
    #
    # "pre-reconcile" is called before Scoby executes the generated object rendering from the reconCiler.
    # "finalization" is called when an object has been deleted.
    capabilities:
    - <HOOK PHASE>
```

- `spec.hook.version` is the Hooks API version that the configured endpoint implements. Must be set to `v1`.
- `spec.hook.address` contains sub elements `uri` and `ref`. When `ref` is informed it should contain an addressable object or a kubernetes service, Scoby will resolve it to an URL and will use it as the hook endpoint. When `uri` is informed it should contain the hook endpoint. If `ref` and `uri` are informed, the kubernetes addressable will be resolved and combined with the scheme, port and path of the `uri`.
- `timeout` is the ISO 8601 duration timeout that the Scoby HTTP client will set when requesting the hook endpoint.
- `capabilities` is an array of the hook implemented capabilities, possible values are `pre-reconcile`, that will be called before Scoby updates any kubernetes object, and `finalize` which would be called before deleting a controlled object.

Upon configured capabilities the hook endpoint will receive requests according to the Hooks API.

## Hooks API v1

At this moment there is only one version of the Hooks API, which must be set at the registration as `v1`. The API supports 2 phases/capabilities, `pre-reconcile` and `finalize`, both using JSON payloads.

### Request and Response

Request and response for both supported phases share the same JSON schema.

A request contains these elements:

```json
{
    "formFactor": "<EITHER deployment OR ksvc>",
    "phase": "<EITHER pre-reconcile OR finalize>",
    "object": "<JSON REPRESENTATION OF RECONCILED OBJECT>",
    "children": {
      "OBJECT1": "<JSON REPRESENTATION OF DESIRED OBJECT>",
      "OBJECT2": "<JSON REPRESENTATION OF DESIRED OBJECT>"
    }
}
```

- `formFactor` identifies the form factor configured at registration. Can be `deployment` or `ksvc`.
- `phase` will be set to `pre-reconcile`.
- `object` is the reconciled object formatted as JSON (including status).
- `children` is the map of the desired kubernetes objects that the form factor generates.

Responses for successful hook scenarios should adhere to this schema:

```json
{
    "object": "<JSON REPRESENTATION OF RECONCILED OBJECT>",
    "children": {
      "OBJECT1": "<JSON REPRESENTATION OF DESIRED OBJECT>",
      "OBJECT2": "<JSON REPRESENTATION OF DESIRED OBJECT>"
    },
}
```

- `object` is the modified reconciled object formatted as JSON (including status).
- `children` is the map of the modified desired kubernetes objects that the form factor generates.

Both elements could be ommited, in which case Scoby will understand that processing can proceed with no changes on kubernetes objects.

Responses for error hook scenarios should return a non 2xx response along with this JSON body:

```json
{
    "message": <ERROR MESSAGE>,
    "permanent": <SHOULD THE ERROR BE RETRIED>,
    "continue": <SHOULD RECONCILE CONTINUE AFTER THIS ERROR>
}
```

- `message` is the error message from the hook.
- `permanent` is an optional boolean value that indicates if the reconciliation should be re-queued. Default value is false.
- `continue` is an optional boolean value that indicates if the current reconciliation cycle should continue. Default value is false.

### Pre-reconcile Phase

At the pre-reconcile phase the hook can implement their own reconciliation logic based on the received object and desired children. If the incoming object needs to be modified, the hook implementation will need to modify the incoming object and return it at the response.
Same goes with the children object, that can be modified and sent back with the response.

- The `object` at the response will only apply changes to the `.status` element.
- The `children` elements will be applied as is, make sure that the hook returns valid objects.
- Not existing or empty `object`/`children` elements will be interpreted as no changes needed from Scoby.

### Finalize phase

When the finalize capatibiliy is declared at the registration, the object will be set a finalizer and on deletion, the finalizer and Scoby created resources will only be removed when the hook's finalize call is successful. There is no use at the finalize phase of the response's `object` and `children` objects.
