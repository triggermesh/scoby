# Migrating TriggerMesh Components

These are practical examples of how we are using Scoby to manage TriggerMesh components at Kubernetes. We have chosen simple components that do not require extra controller logic to avoid using [hooks](../reference/hooks.md).

Some CRD elements containing complex parameters like `spec.adapterOverrides` are not yet migrated, status reporting might show different messages, and some other features like custom reporting at status wont be supported.

:warning: When trying this guide it is important to scale down or remove the TriggerMesh controller to avoid multiple processes fighting to reconcile the same objects.

## Process

For each element migrated in this guide we will follow this steps:

1. Reference the source CRD: the CRD manifests already exists at the [TriggerMesh components repo](https://github.com/triggermesh/triggermesh), we will apply them before registering at Scoby.
2. Create the `ClusterRole`: Scoby will need to be granted read permissions on the CRD instances created by users to manage them. By means of  role aggregations any `ClusterRole` that is labeled `scoby.triggermesh.io/crdregistration: "true"` will be applied to the Scoby controller.
3. Register at Scoby: this require us to go through the expected environment variables at the TriggerMesh adapter and map them with the CRD `.spec` subelements. The image to be used at registration can be found at the repository [releases page](https://github.com/triggermesh/triggermesh/releases)

## Inspecting Components

For each migrated component we will create a table that maps CRDs to environment variables:

| CRD element  | Environment Variable |
|---|---|
| spec.myElement1  | MY_ELEMENT_ONE |
| spec.myElement2  | MY_ELEMENT_TWO |

To gather this information we will need to use [component CRDs](https://github.com/triggermesh/triggermesh/tree/main/config), which are the manifests prefixed with `300-` for sources and `301-` for targets.

The environment variables are defined at the adater code of [sources](https://github.com/triggermesh/triggermesh/tree/main/pkg/sources/adapter) and [targets](https://github.com/triggermesh/triggermesh/tree/main/pkg/targets/adapter). Navigating them you will find at each component a structure like this one:

```go
type envAccessor struct {
  pkgadapter.EnvConfig

  MyElementOne string `envconfig:"MY_ELEMENT_ONE"`
  MyElementTwo int `envconfig:"MY_ELEMENT_TWO"`
}
```

The `pkgadapter.EnvConfig` includes some environment variables that we will be using for all components:

- `NAMESPACE`: Kubernetes namespace where the workload is running.
- `K_METRICS_CONFIG`: JSON configuration for metrics. Refer to [Knative documentation](https://knative.dev/docs/serving/observability/metrics/collecting-metrics/).
- `K_LOGGING_CONFIG`: JSON confdiguration for logging. Refer to [Knative documentation](https://knative.dev/docs/serving/observability/logging/config-logging/).
- `K_SINK`: for sources only, this environment variable must point to a URL where events are being produced to.

While `K_SINK` must be derived from a field specified by the user, the other ones are not. We will add them at the registration using the `.spec.workload.parameterConfiguration.add.toEnv` element. You can replace the empty values with your logging and metrics configuration:

```YAML
spec:
  workload:
    parameterConfiguration:
      add:
        toEnv:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: K_METRICS_CONFIG
          value: "{}"
        - name: K_LOGGING_CONFIG
          value: "{}"
```

For non trivial transformations between the CRD elements and the environment variables we will refer to the reconciler's code where you should find an `adapter.go` file that shows how each element is being serialized. Also at the reconciler we need to make sure if the reconciliation process is executing some extra task like connecting an external API or provisioning resources, in which case we should rely on hooks.

Sources contain an status element that must be informed the resolved URI for the target to which they produce events, that is something we can do with at the `statusConfiguration` using the `valueFrom.path` feature.

```yaml
    statusConfiguration:
      add:
      - path: status.sinkUri
        valueFrom:
          path: spec.sink
```

## WebhookSource Registration

Apply the CRD from [the TriggerMesh repo](https://github.com/triggermesh/triggermesh/blob/main/config/300-webhooksource.yaml):

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/triggermesh/main/config/300-webhooksource.yaml
```

Grant permissions to the controller using the aggregated ClusterRole:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/02.webhooksource/01.webhooksource-clusterrole.yaml
```

Use the CRD reference, a supported image and your form factor of choice for registering:

```yaml
spec:
  crd: webhooksources.sources.triggermesh.io
  workload:
    fromImage:
      repo: gcr.io/triggermesh/webhooksource-adapter:v1.25.0
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
```

| CRD element  | Environment Variable |
|---|---|
| spec.eventType  | WEBHOOK_EVENT_TYPE |
| spec.eventSource  | WEBHOOK_EVENT_SOURCE |
| spec.basicAuthUsername  | WEBHOOK_BASICAUTH_USERNAME |
| spec.basicAuthPassword.(secret)  | WEBHOOK_BASICAUTH_PASSWORD |
| spec.eventExtensionAttributes.from (array) | WEBHOOK_EVENT_EXTENSION_ATTRIBUTES_FROM (comma separated array)|
| spec.corsAllowOrigin | WEBHOOK_CORS_ALLOW_ORIGIN |
| spec.sink (destination) | K_SINK |

Given the CRD element to environment variables table above, add this workload parametrization configuration:

```yaml
    parameterConfiguration:

      fromSpec:
        toEnv:
        - path: spec.eventType
          name: WEBHOOK_EVENT_TYPE
        - path: spec.eventSource
          name: WEBHOOK_EVENT_SOURCE
        - path: spec.basicAuthUsername
          name: WEBHOOK_BASICAUTH_USERNAME
        - path: spec.eventExtensionAttributes.from
          name: WEBHOOK_EVENT_EXTENSION_ATTRIBUTES_FROM
        - path: spec.corsAllowOrigin
          name: WEBHOOK_CORS_ALLOW_ORIGIN
        - path: spec.basicAuthPassword
          name: WEBHOOK_BASICAUTH_PASSWORD
          valueFrom:
            secret:
              name: spec.basicAuthPassword.valueFromSecret.name
              key: spec.basicAuthPassword.valueFromSecret.key
        - path: spec.sink
          name: K_SINK
          valueFrom:
            builtInFunc:
              name: resolveAddress
```

Bundle all those snippets at a YAML file and apply the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/02.webhooksource/02.webhooksource-registration.yaml
```

## KafkaSource Registration

Apply the CRD from [the TriggerMesh repo](https://github.com/triggermesh/triggermesh/blob/main/config/300-kafkasource.yaml):

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/triggermesh/main/config/300-kafkasource.yaml
```

Grant permissions to the controller using the aggregated ClusterRole:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/03.kafkasource/01.kafkasource-clusterrole.yaml
```

Use the CRD reference, a supported image and your form factor of choice for registering:

```yaml
spec:
  crd: kafkasources.sources.triggermesh.io
  workload:
    fromImage:
      repo: gcr.io/triggermesh/kafkasource-adapter:v1.25.0
    formFactor:
      deployment:
        replicas: 1
```

| CRD element  | Environment Variable |
|---|---|
| spec.bootstrapServers  | BOOTSTRAP_SERVERS |
| spec.topic  | TOPIC |
| spec.groupID  | GROUP_ID |
| spec.auth.saslEnable  | SASL_ENABLE |
| spec.auth.securityMechanism  | SECURITY_MECHANISMS |
| spec.auth.tlsEnable  | TLS_ENABLE |
| spec.auth.tls.skipVerify  | SKIP_VERIFY |
| spec.auth.tls.ca (secret)  | CA |
| spec.auth.tls.clientCert (secret) | CLIENT_CERT |
| spec.auth.tls.clientKey (secret) | CLIENT_KEY |
| spec.auth.username  | USERNAME |
| spec.auth.password (secret) | PASSWORD |
| spec.sink (destination) | K_SINK |

Given the CRD element to environment variables table above, add this workload parametrization configuration:

```yaml
    parameterConfiguration:

      fromSpec:
        toEnv:
        - path: spec.bootstrapServers
          name: BOOTSTRAP_SERVERS
        - path: spec.topic
          name: TOPIC
        - path: spec.groupID
          name: GROUP_ID
        - path: spec.auth.saslEnable
          name: SASL_ENABLE
        - path: spec.auth.securityMechanism
          name: SECURITY_MECHANISMS
        - path: spec.auth.tlsEnable
          name: TLS_ENABLE
        - path: spec.auth.tls.skipVerify
          name: SKIP_VERIFY
        - path: spec.auth.tls.ca
          name: CA
          valueFrom:
            secret:
              name: spec.auth.tls.ca.valueFromSecret.name
              key: spec.auth.tls.ca.valueFromSecret.key
        - path: spec.auth.tls.clientCert
          name: CLIENT_CERT
          valueFrom:
            secret:
              name: spec.auth.tls.clientCert.valueFromSecret.name
              key: spec.auth.tls.clientCert.valueFromSecret.key
        - path: spec.auth.tls.clientKey
          name: CLIENT_KEY
          valueFrom:
            secret:
              name: spec.auth.tls.clientKey.valueFromSecret.name
              key: spec.auth.tls.clientKey.valueFromSecret.key
        - path: spec.auth.username
          name: USERNAME
        - path: spec.auth.password
          name: PASSWORD
          valueFrom:
            secret:
              name: spec.auth.password.valueFromSecret.name
              key: spec.auth.password.valueFromSecret.key
        - path: spec.sink
          name: K_SINK
          valueFrom:
            builtInFunc:
              name: resolveAddress
```

Bundle all those snippets at a YAML file and apply the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/03.kafkasource/02.kafkasource-registration.yaml
```

## HTTPTarget Registration

Apply the CRD from [the TriggerMesh repo](https://github.com/triggermesh/triggermesh/blob/main/config/301-httptarget.yaml):

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/triggermesh/main/config/301-httptarget.yaml
```

Grant permissions to the controller using the aggregated ClusterRole:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/05.httptarget/01.httptarget-clusterrole.yaml
```

Use the CRD reference, a supported image and your form factor of choice for registering:

```yaml
spec:
  crd: httptargets.targets.triggermesh.io
  workload:
    fromImage:
      repo: gcr.io/triggermesh/httptarget-adapter:v1.25.0
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
```

| CRD element  | Environment Variable |
|---|---|
| spec.response.eventType | HTTP_EVENT_TYPE |
| spec.response.eventSource | HTTP_EVENT_SOURCE |
| spec.endpoint  | HTTP_URL |
| spec.method  | HTTP_METHOD |
| spec.headers (array) | HTTP_HEADERS (comma separated array) |
| spec.skipVerify | HTTP_SKIP_VERIFY |
| spec.caCertificate | HTTP_CA_CERTIFICATE |
| spec.basicAuthUsername | HTTP_BASICAUTH_USERNAME |
| spec.basicAuthPassword (secret) | HTTP_BASICAUTH_PASSWORD |
| spec.oauthClientID | HTTP_OAUTH_CLIENT_ID |
| spec.oauthClientSecret (secret) | HTTP_OAUTH_CLIENT_SECRET |
| spec.oauthTokenURL | HTTP_OAUTH_TOKEN_URL |
| spec.oauthScopes | HTTP_OAUTH_SCOPE |

Given the CRD element to environment variables table above, add this workload parametrization configuration:

```yaml
    parameterConfiguration:

      fromSpec:
        toEnv:
        - path: spec.response.eventType
          name: HTTP_EVENT_TYPE
        - path: spec.response.eventSource
          name: HTTP_EVENT_SOURCE
          defaultValue: httptarget
        - path: spec.endpoint
          name: HTTP_URL
        - path: spec.method
          name: HTTP_METHOD
        - path: spec.skipVerify
          name: HTTP_SKIP_VERIFY
        - path: spec.caCertificate
          name: HTTP_CA_CERTIFICATE
        - path: spec.basicAuthUsername
          name: HTTP_BASICAUTH_USERNAME
        - path: spec.basicAuthPassword
          name: HTTP_BASICAUTH_PASSWORD
          valueFrom:
            secret:
              name: spec.credentials.name
              key: spec.preferences.key
        - path: spec.oauthClientID
          name: HTTP_OAUTH_CLIENT_ID
  q     - path: spec.oauthClientSecret
          name: HTTP_OAUTH_CLIENT_SECRET
          valueFrom:
            secret:
              name: spec.credentials.name
              key: spec.preferences.key
        - path: spec.oauthTokenURL
          name: HTTP_OAUTH_TOKEN_URL
        - path: spec.oauthScopes
          name: HTTP_OAUTH_SCOPE
        - path: spec.headers
          name: HTTP_HEADERS
```

Bundle all those snippets at a YAML file and apply the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/05.httptarget/02.httptarget-registration.yaml
```

## KafkaTarget Registration

Apply the CRD from [the TriggerMesh repo](https://github.com/triggermesh/triggermesh/blob/main/config/301-kafkatarget.yaml):

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/triggermesh/main/config/301-kafkatarget.yaml
```

Grant permissions to the controller using the aggregated ClusterRole:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/06.kafkatarget/01.kafkatarget-clusterrole.yaml
```

Use the CRD reference, a supported image and your form factor of choice for registering:

```yaml
spec:
  crd: kafkatargets.targets.triggermesh.io
  workload:
    fromImage:
      repo: gcr.io/triggermesh/kafkatarget-adapter:v1.25.0
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
```

| CRD element  | Environment Variable |
|---|---|
| spec.bootstrapServers | BOOTSTRAP_SERVERS |
| spec.topic | TOPIC |
| spec.topicReplicationFactor  | TOPIC_REPLICATION_FACTOR |
| spec.topicPartitions  | TOPIC_PARTITIONS |
| spec.discardCloudEventContext  | DISCARD_CE_CONTEXT |
| spec.auth.saslEnable  | SASL_ENABLE |
| spec.auth.securityMechanism  | SECURITY_MECHANISMS |
| spec.auth.tlsEnable  | TLS_ENABLE |
| spec.auth.tls.skipVerify  | SKIP_VERIFY |
| spec.auth.tls.ca (secret)  | CA |
| spec.auth.tls.clientCert (secret)  | CLIENT_CERT |
| spec.auth.tls.clientKey (secret)  | CLIENT_KEY |
| spec.auth.username  | USERNAME |
| spec.auth.password (secret) | PASSWORD |

Given the CRD element to environment variables table above, add this workload parametrization configuration:

```yaml
    parameterConfiguration:

      fromSpec:
        toEnv:
        - path: spec.bootstrapServers
          name: BOOTSTRAP_SERVERS
        - path: spec.topic
          name: TOPIC
        - path: spec.topicReplicationFactor
          name: TOPIC_REPLICATION_FACTOR
        - path: spec.topicPartitions
          name: TOPIC_PARTITIONS
        - path: spec.discardCloudEventContext
          name: DISCARD_CE_CONTEXT
        - path: spec.auth.saslEnable
          name: SASL_ENABLE
        - path: spec.auth.securityMechanism
          name: SECURITY_MECHANISMS
        - path: spec.auth.tlsEnable
          name: TLS_ENABLE
        - path: spec.auth.tls.skipVerify
          name: SKIP_VERIFY
        - path: spec.auth.tls.ca
          name: CA
          valueFrom:
            secret:
              name: spec.auth.tls.ca.valueFromSecret.name
              key: spec.auth.tls.ca.valueFromSecret.key
        - path: spec.auth.tls.clientCert
          name: CLIENT_CERT
          valueFrom:
            secret:
              name: spec.auth.tls.clientCert.valueFromSecret.name
              key: spec.auth.tls.clientCert.valueFromSecret.key
        - path: spec.auth.tls.clientKey
          name: CLIENT_KEY
          valueFrom:
            secret:
              name: spec.auth.tls.clientKey.valueFromSecret.name
              key: spec.auth.tls.clientKey.valueFromSecret.key
        - path: spec.auth.username
          name: USERNAME
        - path: spec.auth.password
          name: PASSWORD
          valueFrom:
            secret:
              name: spec.auth.password.valueFromSecret.name
              key: spec.auth.password.valueFromSecret.key
```

Bundle all those snippets at a YAML file and apply the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/06.kafkatarget/02.kafkatarget-registration.yaml
```
