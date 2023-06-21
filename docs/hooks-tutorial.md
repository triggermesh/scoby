# Hooks Tutorial

In this guide we will continue using the Kuard application image that we used in the [Scoby tutorial](tutorial.md), will write a go Hook that modifies Scoby behavior, and will configure it using a `CRDRegistration` object.

## Scenario

- We want to programatically set custom environment variables on the Scoby generated workload.
- We want to customize the Scoby generated workload with resource limits. This is something that will be added to Scoby but is not present yet.
- We want to add an extra status to the Kuard object that reflects if the reconciliation from the hook has succeeded or not.
- The Hook will work with deployment and knative services registrations.
- Deletion of an object might be intercepted from the hook and the finaliztion randomly be cancelled.

## The Code

We will be using [go](https://go.dev/), but feel free to use any language where you can create a web server and manage JSON structures.

The code is structured in this blocks:

- The web server that listens for [Hooks v1 APIs](reference/hooks.md#hooks-api-v1).
- The `pre-reconcile` handler (for both deployment and knative service).
  - Children objects management
  - Reconciled Object's status management
- The Deployment `finalize` handler (for both deployment and knative service).

### Web Server

Our web server will be listening at port 8080 and path `v1`. You are free to choose any other port and path since that data can be customizes when registering the hook.

```go
    mux := http.NewServeMux()
    mux.Handle("/v1", h)
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, fmt.Sprintf("no resource at path %q", html.EscapeString(r.URL.String())), http.StatusNotFound)
    })

    srv := http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    if err := srv.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
```

You can see that paths that are not `v1` return a `NotFound` error. It is important that error path return an error code to let Scoby know that the reconciliation did not succeed.

All `v1` API requests are JSON `HookRequests`, inside it we can find the phase the request belongs to and the form factor information.

Form factor information might be important if you want to perform customization on the rendered object. Deployments and knative services have different structures and properties, and when using the deployment form factor it is possible to also render a kubernetes service.

In our handler we first decode the incoming request into a `HookRequest` and redirect to the supported `deployment` and `ksvc` handlers. You only need to implement the form factors that you expect to use.

```go
func (h *HandlerV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    hreq := &hookv1.HookRequest{}
    if err := json.NewDecoder(r.Body).Decode(hreq); err != nil {
        emsg := fmt.Errorf("cannot decode request into HookRequest: %v", err)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
        return
    }


    switch hreq.FormFactor.Name {
    case "deployment":
        h.ServeDeploymentHook(w, hreq)

    case "ksvc":
        h.ServeKsvcHook(w, hreq)

    default:
        emsg := fmt.Errorf("request for formfactor %q not supported", hreq.FormFactor.Name)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
    }
}
```

Focusing on the deployment form factor, there are 2 handlers targeting each phase.

```go
func (h *HandlerV1) ServeDeploymentHook(w http.ResponseWriter, r *hookv1.HookRequest) {
    switch r.Phase {
    case hookv1.PhasePreReconcile:
        h.deploymentPreReconcile(w, r)

    case hookv1.PhaseFinalize:
        h.deploymentFinalize(w, r)

    default:
        emsg := fmt.Errorf("request for phase %q not supported", r.Phase)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
    }
}
```

### Handling Children Objects

At the `pre-reconcile` handler you can navigate the expected children and modify those that you would like to customize. In `go` the simplest way to code this is by converting the incoming children's map into the Kubernetes object.

```go
    ch, ok := r.Children["deployment"]
    if !ok {
        emsg := errors.New("children deployment element not found")
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
        return
    }

    d := &appsv1.Deployment{}
    if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ch.Object, d); err != nil {
        emsg := fmt.Errorf("malformed deployment at children element: %w", err)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
        return
    }
```

In the case above we explored the request's children map looking for the `deployment` object rendered from Scoby, and converted it into a Kubernetes object.

Let's start setting an environment variable from our hook.

```go
    cs := d.Spec.Template.Spec.Containers
    if len(cs) == 0 {
        emsg := errors.New("children deployment has no containers")
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
        return
    }

    cs[0].Env = append(cs[0].Env,
        corev1.EnvVar{
            Name:  "FROM_HOOK_VAR",
            Value: "this value is set from the hook",
        })
```

Just like we did above with the environment variable, we can manage any Deployment property. Let's add some resource request and limits to the container.

```go
    cs[0].Resources = corev1.ResourceRequirements{
        Requests: corev1.ResourceList{
            corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
            corev1.ResourceMemory: *resource.NewQuantity(1024*1024*100, resource.BinarySI),
        },
        Limits: corev1.ResourceList{
            corev1.ResourceCPU: *resource.NewMilliQuantity(250, resource.DecimalSI),
        },
    }
```

:warning: It is important that when the Hook reconciles and object it always renders the same output for the children objects.

Now we serialize the new deployment object and add it to the hook response structure. Scoby will use the returned deployment with our changes.

```go
    u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
    if err != nil {
        emsg := fmt.Errorf("could not convert modified deployment into unstructured: %w", err)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusBadRequest)
        return
    }

    r.Children["deployment"] = &unstructured.Unstructured{Object: u}

    hres := &hookv1.HookResponse{
        Children: r.Children,
    }

    if err := json.NewEncoder(w).Encode(hres); err != nil {
        emsg := fmt.Errorf("error encoding response: %w", err)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
```

### Handling Object Status

If the hook fails ro succeeds to reconcile an object, its status should reflect this condition. Hooks allow you to declare and manage status conditions at will. In our scenario we will have only a status being managed by the hook but you can declare as many as you might need.

To manage the reconciled object's status we need to use the hook request's object.

Continuing with the code for customizing the deployment's container, we can update the status before serializing the hook response.

```go
    if err := h.setHookStatusPreReconcile(w, &r.Object, string(corev1.ConditionTrue), ConditionReasonOK); err != nil {
        emsg := fmt.Errorf("error setting object status: %w", err)
        logError(emsg)
        http.Error(w, emsg.Error(), http.StatusInternalServerError)
        return
    }

    hres := &hookv1.HookResponse{
        Object:   &r.Object,
        Children: r.Children,
    }
```

Notice the incoming Object element being re-used for the hook response. We pass a reference to that Object at the  `setHookStatusPreReconcile` method, that will modify its status element. In our case, we use the [go apimachinery's unstructured](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured) package to navigate the status and set a `HookReportedStatus` condition that we will declare when registering at Scoby.

```go
const (
    ConditionType     = "HookReportedStatus"
    ConditionReasonOK = "HOOKREPORTSOK"
)

...

    if conditions, ok, _ := unstructured.NestedSlice(u.Object, "status", "conditions"); ok {
        var hookCondition map[string]interface{}

        // Look for existing condition
        for i := range conditions {
            c, ok := conditions[i].(map[string]interface{})
            if !ok {
                return fmt.Errorf("wrong condition format: %+v", conditions[i])
            }

            t, ok := c["type"].(string)
            if !ok {
                return fmt.Errorf("wrong condition type: %+v", c["type"])
            }

            if ok && t == ConditionType {
                hookCondition = conditions[i].(map[string]interface{})
                break
            }
        }

        // If the condition does not exist, create it.
        if hookCondition == nil {
            hookCondition = map[string]interface{}{
                "type": ConditionType,
            }
            conditions = append(conditions, hookCondition)
        }

        hookCondition["status"] = status
        hookCondition["reason"] = reason

        if err := unstructured.SetNestedSlice(u.Object, conditions, "status", "conditions"); err != nil {
            return err
        }
    }
```

In the code abover we look for the hook condition and set it to the desired values, then add it to the object.

If there are custom fields at the status of your CRD you can also manage them just as we did for the status condition.

```go
    annotations, ok, _ := unstructured.NestedStringMap(u.Object, "status", "annotations")
    if !ok {
        annotations = map[string]string{}
    }

    annotations["greetings"] = "from hook"

    return unstructured.SetNestedStringMap(u.Object, annotations, "status", "annotations")
```

### Handling Finalization

When a Scoby `CRDRegistration` contains a hook that declares the `finalize` capability, the hook will be contacted before deleting the object and children, and if an error response is returned, finalization will not occur.

## Deployment

Our hook will be created as a Deployment and a Service, the latest being configured as the endpoint at the CRD Registration. The

## Registration

- [Kuard repository](https://github.com/kubernetes-up-and-running/kuard)
- Kuard image: gcr.io/kuar-demo/kuard-amd64:blue

## Testing It

A very generic playground [kuard CRD](https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/01.kuard-crd.yaml) can be found at Scoby repo, containing combination of nested elements, arrays, object references and [full status support for Scoby](reference/status.md).

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

The Kuard CRD created above needs to be managed by the Scoby controller. An aggregated `ClusterRole` provides the mechanism to grant those permissions by tagging a `ClusterRole` with the `scoby.triggermesh.io/crdregistration: true` label.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/02.kuard-clusterrole.yaml
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

![scoby summary](assets/kuard-deployment-envs.png)

If the registered CRD constains a `status.address.url` element, and it renders a Kubernetes service or Knative service, the internal address is populated at the aforementioned element.

```console
kubectl get kuard my-kuard-extension -ojsonpath='{.status}' | jq .
```

```json
{
  "address": {
    "url": "http://my-kuard-extension.default.svc.cluster.local"
  },
  "conditions": [
    {
      "lastTransitionTime": "2023-03-23T13:09:44Z",
      "message": "",
      "reason": "MinimumReplicasAvailable",
      "status": "True",
      "type": "DeploymentReady"
    },
    {
      "lastTransitionTime": "2023-03-23T13:09:44Z",
      "message": "",
      "reason": "CONDITIONSOK",
      "status": "True",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2023-03-23T13:09:44Z",
      "message": "",
      "reason": "ServiceExist",
      "status": "True",
      "type": "ServiceReady"
    }
  ],
  "observedGeneration": 1
}
```

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
      fromSpec:
      # Skip variable2 from generating a parameter for the workload
        skip:
        - path: spec.variable2


```

The `spec.workload.parameterConfiguration.fromSpec[].skip` boolean indicates whether the environment variable for the element should be generated.

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

Most often expected environment variables at the container do not match Scoby's automatic rendering. All generated environment variables can be renamed using `spec.workload.parameterConfiguration.fromSpec[].toEnv.name`.

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
      fromSpec:
        toEnv:
        # Rename variable2
        - path: spec.variable2
          name: KUARD_VARIABLE_TWO
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

### Parameter Default Value

The value for an environment variable can be set to a default value using `spec.workload.parameterConfiguration.fromSpec[].toEnv.defaultValue`.

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
      fromSpec:
        toEnv:
        # Override variable2 value
        - path: spec.variable2
          defaultValue: new variable2 value
```

Create the registration:

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/01.kuard/05.param.value/01.kuard-registration.yaml
```

Create a mutation fo the instance we have created so far that doesn't inform `spec.value2`:

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

Note the variable at `.spec.variable2` value has been defaulted.
Clean up the example:

```console
kubectl delete kuard my-kuard-extension
kubectl delete crdregistration kuard
```

### Parameter Value From Secret

The value for an environment variable can reference a Secret through the `spec.workload.parameterConfiguration.fromSpec[].toEnv.valueFromSecret` customization option, that needs the `name` and `key` subelements to be set. In this example we will also be setting the variable name.

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
      fromSpec:
        toEnv:
          # Reference a secret
          - path: spec.refToSecret
          name: FOO_CREDENTIALS
          valueFrom:
            secret:
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

The value for an environment variable can reference a ConfigMap through the `spec.workload.parameterConfiguration.fromSpec[].toEnv.valueFromConfigMap` customization option, that needs the `name` and `key` subelements to be set.

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
      fromSpec:
        toEnv:
        # Reference a ConfigMap
        - path: spec.refToConfigMap
          valueFrom:
            configMap:
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

Use the built-in function `spec.workload.parameterConfiguration.fromSpec[].toEnv.valueFromBuiltInFunc.resolveAddress` on the element that contains the Destination type. As an added feature this example also updates an status element with the resolved address.

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
      fromSpec:
        toEnv:
        # Resolve an address
        - path: spec.refToAddress
          name: FOO_SINK
          valueFrom:
            builtInFunc:
              name: resolveAddress
    statusConfiguration:
      add:
      # Add the result to an status element
      - path: status.sinkUri
        valueFrom:
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

The `status.sinkUri` element has been filled with the value from the resolved address above.

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

In scenarios where parameters unrelated to the instance `.spec` data needs to be added, the `spec.workload.parameterConfiguration.add.Envs[]` is used. An array of [EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#envvar-v1-core) can be provided referencing literal values, ConfigMaps, Secrets or the Downward API.

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
      add:
        toEnv:
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

Remove the registered CRD and ClusterRole.

```console
kubectl delete clusterrole crd-registrations-scoby-kuard
kubectl delete crd kuards.extensions.triggermesh.io
```

**Important Note**: Due to limitations at controller-runtime, removing a CRD that has been watched will lead to logging errors at the still existing informer. That can be solved restarting the informer and will be solved after [this issue](https://github.com/kubernetes-sigs/controller-runtime/issues/1884) is solved.
