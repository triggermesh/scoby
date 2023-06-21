# Hooks Tutorial

In this guide we will continue using the Kuard application image that we used in the [Scoby tutorial](tutorial.md), will write a go Hook that modifies Scoby behavior, and will configure it using a `CRDRegistration` object.

Note: Scoby must be installed before running the hook sample.

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

The full code is available [here](https://github.com/triggermesh/scoby/blob/main/cmd/kuard-hook-sample/main.go)

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

If there are custom fields at the status of your CRD you can also manage them just as we did for the status condition. Here we add an annotation item.

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

We will randomly return an outcome for the finalization. Either we reply with an empty response and a 200 code, meaning finalization can proceed, or we return an structured error and a 500 code, meaning the finalization must be blocked.

In real world scenarios you will replace this random behavior with some clean-up tasks.

```go
    if h.rnd.Intn(2) == 0 {
        log.Printf("canceling finalization ...")

        w.WriteHeader(http.StatusInternalServerError)
        w.Header().Set("Content-Type", "application/json")

        _false := false
        herr := &hookv1.HookResponseError{
            Message:   "finalization denied from hook",
            Permanent: &_false,
        }

        if err := json.NewEncoder(w).Encode(herr); err != nil {
            emsg := fmt.Errorf("error encoding response: %w", err)
            logError(emsg)
            http.Error(w, emsg.Error(), http.StatusInternalServerError)
        }
    }
```

When deleting an object the hook will generate a random integer that can be 0 or 1. When 0 the finalization will be denied to Scoby, and the object won't be deleted.
The `permanent` flag of the error response is set to false, this will re-queue the reconciliation and a new attempt will be made until the hook allows the finalization to occur.

## Deployment

Scoby registration will need to be informed the address of the hook. We will use an in-cluster Deployment and a Service, the latest being configured as the endpoint at the CRD Registration.

To deploy a pre-compiled image launc this command.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/07.kuard-hook/00.hook.yaml
```

To deploy from code using [ko](https://github.com/ko-build/ko), checkout this repo and run:

```console
ko apply -f docs/samples/07.kuard-hook/00.ko-hook.yaml
```

## Registration

As we did at the main tutorial, before registering we need to:

- create the kuard CRD.
- grant Scoby permissions to manage kuard instances

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/07.kuard-hook/01.kuard-crd.yaml

kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/07.kuard-hook/02.kuard-clusterrole.yaml
```

The registration contains 2 blocks of information regarding hooks. On one side the `spec.hook` element contains hook information like the address and its capabilities.

```yaml
apiVersion: scoby.triggermesh.io/v1alpha1
kind: CRDRegistration
metadata:
  name: kuard
spec:
  crd: kuards.extensions.triggermesh.io
  hook:
    version: v1
    address:
      uri: "http://:8080/v1"
      ref:
        apiVersion: v1
        kind: Service
        name: scoby-hook-kuard
        namespace: triggermesh

    capabilities:
    - pre-reconcile
    - finalize
```

We are using `spec.hook.address.ref` configuring the kubernets service created at the previous step as the hook's endpoint, and modifying the port and path via the `spec.hook.address.uri`.

At the `spec.workload.statusConfiguration` section we can tell Scoby that a new status must be added to the reconciled object and that the hook will take care of it.

```yaml
  workload:
    formFactor:
      deployment:
        replicas: 1
        service:
          port: 80
          targetPort: 8080
    fromImage:
      repo: gcr.io/kuar-demo/kuard-amd64:blue

    statusConfiguration:
      conditionsFromHook:
      - type: HookReportedStatus
```

Apply the full registration using this command.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/07.kuard-hook/03.kuard-registration-hook.yaml
```

## Testing

Any kuard instance will now be pre-rendered by Scoby, then passed to the configured hook where:

- an environment variable is added.
- resources requests and limits are defined.
- a status condition is set.

```console
kubectl apply -f https://raw.githubusercontent.com/triggermesh/scoby/main/docs/samples/07.kuard-hook/04.kuard-instance.yaml
```

List the status conditions for the kuard object.

```console
kubectl get kuard my-kuard-extension -o jsonpath='{.status.conditions}' | jq
```

Note the `HookReportedStatus` filled from the hook.

```json
[
  {
    "lastTransitionTime": "2023-06-21T12:34:11Z",
    "message": "",
    "reason": "MinimumReplicasAvailable",
    "status": "True",
    "type": "DeploymentReady"
  },
  {
    "lastTransitionTime": "2023-06-21T12:34:09Z",
    "message": "",
    "reason": "HOOKREPORTSOK",
    "status": "True",
    "type": "HookReportedStatus"
  },
  {
    "lastTransitionTime": "2023-06-21T12:34:11Z",
    "message": "",
    "reason": "CONDITIONSOK",
    "status": "True",
    "type": "Ready"
  },
  {
    "lastTransitionTime": "2023-06-21T12:34:11Z",
    "message": "",
    "reason": "ServiceExist",
    "status": "True",
    "type": "ServiceReady"
  }
]
```

Get the status annotations.

```console
kubectl get kuard my-kuard-extension -o jsonpath='{.status.annotations}' | jq
```

The annotation that we wrote at the hook's code should show up.

```json
{
  "greetings": "from hook"
}
```

Inspecting the deployment that was created we should find the hook's environment variable.

```console
kubectl get deployments.apps -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.template.spec.containers[0].env}' | jq
```

```json
[
  {
    "name": "VARIABLE1",
    "value": "value 1"
  },
  {
    "name": "FROM_HOOK_VAR",
    "value": "this value is set from the hook"
  }
]
```

Checking resources at the deployment's container also shows the values set from the hook.

```console
kubectl get deployments.apps -l app.kubernetes.io/name=kuard -ojsonpath='{.items[0].spec.template.spec.containers[0].resources}' | jq
```

```json
{
  "limits": {
    "cpu": "250m"
  },
  "requests": {
    "cpu": "100m",
    "memory": "100Mi"
  }
}
```

When deleting the Kuard instance, the action might be delayed by the hook. To check that we will need to leave a shell open following the hook's logs.

```console
 kubectl logs -n triggermesh -l app.kubernetes.io/component=scoby-hook-kuard -f --tail 0
```

On a different shell delete the instance.

```console
kubectl delete kuard my-kuard-extension
```

The hook might randomly cancel finalization. You can try to create and delete kuard instances a number of times and see how the hook logs its behavior. At the logs below you can see that it canceled the first finalization cycle and allowed the second.

```console
kubectl logs -n triggermesh -l app.kubernetes.io/component=scoby-hook-kuard -f --tail 0
2023/06/21 12:51:55 finalize deployment
2023/06/21 12:51:55 canceling finalization ...
2023/06/21 12:51:55 finalize deployment
2023/06/21 12:51:55 hook says yes to finalization
```

## Clean Up

Remove the registered CRD and ClusterRole.

```console
kubectl delete clusterrole crd-registrations-scoby-kuard
kubectl delete crd kuards.extensions.triggermesh.io
kubectl delete deployment -n triggermesh scoby-hook-kuard
kubectl delete service -n triggermesh scoby-hook-kuard
```
