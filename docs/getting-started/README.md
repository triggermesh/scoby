# Getting Started

In this guide we will use the Kuard application image which was created by the authors of Kubernetes Up and Ready book, and is able to render a list of its environment variables:

- [Kuard repository](https://github.com/kubernetes-up-and-running/kuard)
- Kuard image: gcr.io/kuar-demo/kuard-amd64:blue

A very generic playground [kuard CRD](../../docs/samples/01.kuard/01.kuard-crd.yaml) can be found at Scoby repo, containing combination of nested elements, arrays, object references and [full status support for Scoby](../status.md).

A flattened version of that CRD `spec` contents would look like this:

```text
spec.variable1
spec.variable2
spec.group.variable3
spec.group.variable4
spec.array[]
spec.reftoSecret.secretName
spec.reftoSecret.secretKey
spec.refToConfigMap.configName
spec.refToConfigMap.configKey
spec.refToAddress.uri
spec.refToAddress.ref.apiVersion
spec.refToAddress.ref.kind
spec.refToAddress.ref.name
spec.refToAddress.ref.namespace
```

## Initialization

For this guide we will create the CRD once, and use it with different registrations, resulting in different workloads settings for each of them:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/01.kuard-crd.yaml
```

The results of each registration at this guide can be check by port forwarding the generated workload and navigating kuard, or by inspecting the generated pod's environment variables.

- Navigating kuard: forward the generated service for a deployment. In the case of a Knative Serving service, use the external address to access the UI.

```console
kubectl port-forward svc/my-kuard-extension  8888:80
```

- Inspecting pod's environment variables: replace the pod name with the Scoby rendered pod.

```console
kubectl get po my-rendered-pod -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

We will stick to the pod inspecting method but for the first example, where we will use both.

### Deployment Registration

The deployment registration is going to be used for most examples at this guide due to not requiring any added software compared with the Knative Service registration. The form factor at this example is configured to create one pod and a service that listens on 80 and forward requests to the pod's 8080, where the kuard application is listening:

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
  workload:
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
    fromImage:
      repo: gcr.io/kuar-demo/kuard-amd64:blue
```

You can also find in the YAML snippet above the reference to the CRD and image that will remain constant throughout all examples.

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/01.deployment/01.kuard-registration.yaml
```

Scoby spins up a controller that will manage `kuard01` objects. Let's create an instance, and to get started with the default rendering behavior, let's fill some elements in it:

```yaml
apiVersion: extensions.triggermesh.io/v1
kind: Kuard
metadata:
  name: my-kuard-extension
spec:
  variable1: value 1
  variable2: value 2
  group:
    variable3: false
    variable4: 42
  array:
  - alpha
  - beta
  - gamma
```

The spec above matches a subset of the CRD structure. Only these fields will be converted into environment variables when applied.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/01.deployment/02.kuard-instance.yaml
```

A deployment and a service must have been generated:

```console
kubectl get deployment,svc -l app.kubernetes.io/name=kuard
```

Retrieve the pod's environment variables.
Note: using the label selector return a list, we expect a single pod to match it at `items[0]`:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

The result shows Scoby rendering each informed element as environment variables whose name is a capitalized concatenation of the element hierarchy:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  }
]
```

Exploring kuard's interface we can also find these environment variables:

```console
kubectl port-forward svc/my-kuard-extension  8888:80
```

![scoby summary](../assets/kuard-deployment-envs.png)

Let's remove the kuard instance and registration before proceeding with the next one:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

This will remove the kuard instance and its generated components, and the Scoby registration.

### Knative Serving Registration

The Knative Service registration requires Knative Serving to be installed. Follow the [instructions at the Knative site](https://knative.dev/docs/install/) to install it. The form factor at this example is configured to create a public service that scales between 1 and 3 instances. You can set the `minScale` parameter to 0 to enable `scale to 0` feature:

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
  workload:
    formFactor:
      knativeService:
        minScale: 1
        maxScale: 3
        visibility: public
    fromImage:
      repo: gcr.io/kuar-demo/kuard-amd64:blue
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/02.knative-service/01.kuard-registration.yaml
```

Now create the same kuard instance we created for the deployment registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/02.knative-service/02.kuard-instance.yaml
```

The service generates a pod whose environment variables can be inspected using a Knative Service version of the label filter that we used for the deployment:

```console
kubectl get po -l serving.knative.dev/service=my-kuard-extension -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

You can find some Knative Serving variables being added, and the same Scoby variables we got at the Deployment form factor example.

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  },
  {
    "name": "PORT",
    "value": "8080"
  },
  {
    "name": "K_REVISION",
    "value": "my-kuard-extension-00001"
  },
  {
    "name": "K_CONFIGURATION",
    "value": "my-kuard-extension"
  },
  {
    "name": "K_SERVICE",
    "value": "my-kuard-extension"
  }
]
```

Let's clean up the example.

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Skip Parameter Rendering

When an element in the spec is not meant to generate an environment variable, the rendering can be skipped via a configuration parameter.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Skip variable2 from generating a parameter for the workload
      - path: spec.variable2
        render:
          skip: true

```

The `spec.workload.parameterConfiguration.customize[].render.skip` boolean indicates whether the environment variable for the element should be generated.

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/03.param.skip/01.kuard-registration.yaml
```

Create the same instance we have created so far:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/03.param.skip/02.kuard-instance.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  }
]
```

Rendering skipped `.spec.variable2` rendering.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Parameter Renaming

Most often expected environment variables at the container do not match Scoby's automatic rendering. All generated environment variables can be renamed using `spec.workload.parameterConfiguration.customize[].render.key`.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Rename variable2
      - path: spec.variable2
        render:
          key: KUARD_VARIABLE_TWO
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/04.param.rename/01.kuard-registration.yaml
```

Create the same instance we have created so far:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/04.param.rename/02.kuard-instance.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "KUARD_VARIABLE_TWO",
    "value": "value 2"
  }
]
```

Note the variable at `.spec.variable2` renamed as `KUARD_VARIABLE_TWO`.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Parameter Value Literal

The value for an environment variable can be set to a literal value using `spec.workload.parameterConfiguration.customize[].render.value`.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Override variable2 value
      - path: spec.variable2
        render:
          value: new variable2 value
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/05.param.value/01.kuard-registration.yaml
```

Create the same instance we have created so far:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/05.param.value/02.kuard-instance.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "new variable2 value"
  }
]
```

Note the variable at `.spec.variable2` value has been overriden.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Parameter Value From Secret

The value for an environment variable can reference a Secret through the `spec.workload.parameterConfiguration.customize[].render.valueFromSecret` customization option, that needs the `name` and `key` subelements to be set. In this example we will also be setting the variable name.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Reference a secret
      - path: spec.refToSecret
        render:
          key: FOO_CREDENTIALS
          valueFromSecret:
            name: spec.refToSecret.secretName
            key: spec.refToSecret.secretKey
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/06.param.secret/01.kuard-registration.yaml
```

Create the same instance we have created so far plus the Secret it references:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/06.param.secret/02.kuard-instance.yaml
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/06.param.secret/03.secret.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "FOO_CREDENTIALS",
    "valueFrom": {
      "secretKeyRef": {
        "key": "kuard-key",
        "name": "kuard-secret"
      }
    }
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  }
]
```

Note the variable at `.spec.refToSecret` is rendered with name `FOO_CREDENTIALS` as a reference to a secret.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
kubectl delete secret kuard-secret
```

### Parameter Value From ConfigMap

The value for an environment variable can reference a ConfigMap through the `spec.workload.parameterConfiguration.customize[].render.valueFromConfigMap` customization option, that needs the `name` and `key` subelements to be set.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Reference a ConfigMap
      - path: spec.refToConfigMap
        render:
          valueFromConfigMap:
            name: spec.refToConfigMap.configName
            key: spec.refToConfigMap.configKey
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/07.param.configmap/01.kuard-registration.yaml
```

Create the same instance we have created so far plus the ConfigMap it references:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/07.param.configmap/02.kuard-instance.yaml
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/07.param.configmap/03.configmap.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "REFTOCONFIGMAP",
    "valueFrom": {
      "configMapKeyRef": {
        "key": "kuard-key",
        "name": "kuard-configmap"
      }
    }
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  }
]
```

Note the variable at `.spec.refToConfigMap` is rendered with name `REFTOCONFIGMAP` as a reference to a ConfigMap.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
kubectl delete configmap kuard-configmap
```

### Parameter Value From Function: resolveAddress

If part of a spec uses a [Destination duck type](https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Destination) to express a location, just like [Knative Sinks](https://knative.dev/docs/eventing/sinks/#sink-as-a-parameter) do, the registration can be used to resolve it and use the result as an environment variable.

A destination duck type informs either an URI, a Kubernetes service, or a Kubernetes object that contains a URL at `status.address.url`.

```yaml
  destination:
    ref:
      apiVersion: <version>
      kind: <kind>
      namespace: <namespace - optional>
      name: <name>
    uri: <uri>
```

Use the built-in function `spec.workload.parameterConfiguration.customize[].valueFromBuiltInFunc.resolveAddress` on the element that contains the Destination type. As an added feature this example also updates an status element with the resolved address.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      customize:
      # Resolve an address
      - path: spec.refToAddress
        render:
          key: FOO_SINK
          valueFromBuiltInFunc:
            name: resolveAddress
    statusConfiguration:
      addElements:
      # Add the result to an status element
      - path: status.sinkUri
        render:
          valueFromParameter:
            path: spec.refToAddress
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/08.param.addressable/01.kuard-registration.yaml
```

Create a service or addressable and reference it from a kuard instance:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/08.param.addressable/02.kuard-instance.yaml
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/08.param.addressable/03.service.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "FOO_SINK",
    "value": "http://my-service.default.svc.cluster.local"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  }
]
```

Note the variable at `.spec.refToAddress` is rendered with name `FOO_SINK` containing the DNS address for the service.
Also check the status:

```console
kubectl get kuard my-kuard-extension -ojsonpath='{.status}' | jq .
```

The `status.address.url` element has been filled with the value from the resolved address above.

```yaml
{
  "address": {
    "url": "http://my-kuard-extension.default.svc.cluster.local"
  },
  "conditions": [
    {
      "lastTransitionTime": "2023-03-21T09:40:38Z",
      "message": "",
      "reason": "MinimumReplicasAvailable",
      "status": "True",
      "type": "DeploymentReady"
    },
    {
      "lastTransitionTime": "2023-03-21T09:40:38Z",
      "message": "",
      "reason": "CONDITIONSOK",
      "status": "True",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2023-03-21T09:40:38Z",
      "message": "",
      "reason": "ServiceExist",
      "status": "True",
      "type": "ServiceReady"
    }
  ],
  "observedGeneration": 1,
  "sinkUri": "http://my-service.default.svc.cluster.local"
}
```

Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
kubectl delete service my-service
```

### Add New Parameter

In scenarios where parameters unrelated to the instance `.spec` data needs to be added, the `spec.workload.parameterConfiguration.addEnvs[]` is used. An array of [EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#envvar-v1-core) can be provided referencing literal values, ConfigMaps, Secrets or the Downward API.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      # A new variable will be created using a reference to the object's field.
      addEnvs:
      - name: MY_POD_NAME
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/09.param.add.metadata/01.kuard-registration.yaml
```

Create a kuard instance:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/09.param.add.metadata/02.kuard-instance.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "MY_POD_NAME",
    "valueFrom": {
      "fieldRef": {
        "apiVersion": "v1",
        "fieldPath": "metadata.name"
      }
    }
  },
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "VARIABLE2",
    "value": "value 2"
  }
]
```

Note the variable named `MY_POD_NAME` using the Downward API.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Global Parameter Prefix

Generated environment variables names can be added a prefix by using the `spec.workload.parameterConfiguration.global.defaultPrefix` element. All generated names will use the prefix but for those that contain extra configuration that set a key name.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
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
      # Use a prefix for all generated variables.
      global:
        defaultPrefix: KUARD_
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/10.param.prefix/01.kuard-registration.yaml
```

Create a kuard instance:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/10.param.prefix/02.kuard-instance.yaml
```

Inspect generated environment variables:

```console
kubectl get po -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.containers[0].env}' | jq .
```

Look at the result:

```json
[
  {
    "name": "KUARD_ARRAY",
    "value": "alpha,beta,gamma"
  },
  {
    "name": "KUARD_GROUP_VARIABLE3",
    "value": "false"
  },
  {
    "name": "KUARD_GROUP_VARIABLE4",
    "value": "42"
  },
  {
    "name": "KUARD_VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "KUARD_VARIABLE2",
    "value": "value 2"
  }
]
```

Note that each key has been prefixed with `KUARD_`.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

## Clean Up

Remove the registered CRD. Note: the controller is still not able to remove informers, logs will complain about the CRD not being present. This can only be solved at the moment restarting the controller.

```console
kubectl delete crd kuards.extensions.triggermesh.io
```

**Important Note**: Due to limitations at controller-runtime, removing a CRD that has been watched will lead to logging errors at the still existing informer. That can be solved restarting the informer and will be solved after [this issue](https://github.com/kubernetes-sigs/controller-runtime/issues/1884) is solved.
