# Status Management

Registered CRDs can see their instances status filled if their structure matches Scoby expectations.

## CRD Status

CRD Status are inspected when registering to check it support for:

- Address URL: when rendering a kubernetes service or knative service as the workload's form factor, the address element is populated with the cluster reachable URL.

```yaml
status:
  type: object
  properties:
    address:
      type: object
      properties:
        url:
          type: string
```

- Annotations: allow setting status annotations. Not used by Scoby yet.

```yaml
status:
  type: object
  properties:
    annotations:
      type: object
      additionalProperties:
        type: string
```

- Conditions: conditions are checked for these subelements
  - type: the name that identifies the condition.
  - status: the status for the condition, one of `True`, `False` or `Unknown`.
  - reason: identifier that explains the reason why the condition type is set to a status.
  - message: human readable message that provides further information on non `True` statuses.
  - lastTransitionTime: time the conditions where last updated.

```yaml
status:
  type: object
  properties:
    conditions:
      type: array
      items:
        type: object
        properties:
          type:
            type: string
          status:
            type: string
            enum: ['True', 'False', Unknown]
          reason:
            type: string
          message:
            type: string
          lastTransitionTime:
            type: string
       required:
       - type
       - status
```

- Observed Generations: if existing the object's generation will be set as the observed generation at the status after reconciling.

```yaml
status:
  type: object
  properties:
    observedGeneration:
      type: integer
```
