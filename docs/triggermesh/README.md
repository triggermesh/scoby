# Migrating TriggerMesh Components

This is a practical example of how we are using Scoby to manage TriggerMesh components at Kubernetes. We have chosen simple components that do not require extra controller logic to avoid using [hooks](../hooks.md).

Some CRD elements like `spec.adapterOverrides` are not yet supported by Scoby.

When trying this guide it is important to scale down or remove the TriggerMesh controller to avoid multiple processes fighting to reconcile the same objects.

## WebhookSource

The CRD for this source can be found [here](https://github.com/triggermesh/triggermesh/blob/main/config/300-webhooksource.yaml).

Apply the CRD:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/triggermesh/main/config/300-webhooksource.yaml
```

Before registering the CRD at Scoby we need to grant permissions to the controller using the aggregated ClusterRole label and letting it read the source and update the status.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/02.webhooksource/01.webhooksource-clusterrole.yaml
```

The image can be found at the repository [releases page](https://github.com/triggermesh/triggermesh/releases). The registration using a Kubernetes deployment and service would look like this:

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: webhooksource
spec:
  crd: webhooksources.sources.triggermesh.io
  workload:
    fromImage:
      repo: gcr.io/triggermesh/webhooksource-adapter:v1.24.4
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
```

Replace the `deployment` section with a `knativeService` when using Knative.

The schema contains all object fields under the `.spec` root element, and the `.status` element contains `conditions` array and a `sinkUri` to host the resolved URI to send events to.

This is the mapping from CRD elements to the `WebhookSource` adapter application expected environment variables:

| CRD element  | Environment Variable |
|---|---|
| spec.eventType  | WEBHOOK_EVENT_TYPE |
| spec.eventSource  | WEBHOOK_EVENT_SOURCE |
| spec.basicAuthUsername  | WEBHOOK_BASICAUTH_USERNAME |
| spec.basicAuthPassword.(secret)  | WEBHOOK_BASICAUTH_PASSWORD |
| spec.eventExtensionAttributes.from (array) | WEBHOOK_EVENT_EXTENSION_ATTRIBUTES_FROM |
| spec.corsAllowOrigin | WEBHOOK_CORS_ALLOW_ORIGIN |
| spec.sink (destination) | K_SINK |

Primitive values need no special treatment. Also the `spec.eventExtensionAttributes.from` array, which produces CloudEvents attributes from the HTTP request, is expected to be a comma separated string at the environment variable, and that is the default rendering at Scoby for an array, hence can be added to the registration.

```yaml
    parameterConfiguration:

      customize:
      - path: spec.eventType
        render:
          name: WEBHOOK_EVENT_TYPE

      - path: spec.eventSource
        render:
          name: WEBHOOK_EVENT_SOURCE

      - path: spec.basicAuthUsername
        render:
          name: WEBHOOK_BASICAUTH_USERNAME

      - path: spec.eventExtensionAttributes.from
        render:
          name: WEBHOOK_EVENT_EXTENSION_ATTRIBUTES_FROM

      - path: spec.corsAllowOrigin
        render:
          name: WEBHOOK_CORS_ALLOW_ORIGIN

```

HTTP basic authentication password is informed using a secret. Use the `valueFromSecret` element at the registration to point at the secret name and key that hosts the user's password.

```yaml
      - path: spec.basicAuthPassword
        render:
          name: WEBHOOK_BASICAUTH_PASSWORD
          valueFromSecret:
            name: spec.basicAuthPassword.valueFromSecret.name
            key: spec.basicAuthPassword.valueFromSecret.key
```

The `spec.sink` is a filed that can inform either a URI or an object that should be resolved to an URI. This is a non straighforward task at the controller, usually this kind of operations require hooks to be called from Scoby but for this one we created a built-in function called `resolveAddress`.

```yaml
      - path: spec.sink
        render:
          name: K_SINK
          valueFromBuiltInFunc:
            name: resolveAddress
```

The resolved URI should be used as a value at the `status.sinkUri`, which is something we can do with at the `statusConfiguration` using the `valueFromParameter` feature.

```yaml
    statusConfiguration:
      addElements:
      - path: status.sinkUri
        render:
          valueFromParameter:
            path: spec.sink
```

Bundle all those snippets at a YAML file and apply the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/02.webhooksource/01.webhooksource-registration.yaml
```

## Usage

Users can now create `WebhookSource` objects, Scoby will do the reconciliation, resolve the sink URI, create the required workload using the parameters that we registered, and will reflect the provisioning outcome at the status:

```yaml
apiVersion: sources.triggermesh.io/v1alpha1
kind: WebhookSource
metadata:
  name: sample
spec:
  eventType: com.example.mysample.event
  eventSource: instance-abc123

  eventExtensionAttributes:
    from:
    - path
    - queries

  sink:
    ref:
      apiVersion: eventing.triggermesh.io/v1alpha1
      kind: RedisBroker
      name: demo
```
