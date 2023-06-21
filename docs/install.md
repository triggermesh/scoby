# Install

To install Scoby at a Kubernetes cluster apply manifests for both CRDs and Controller:

```console
# Install Scoby CRDs
kubectl apply -f https://github.com/triggermesh/scoby/releases/latest/download/scoby-crds.yaml

# Install Scoby Controller
kubectl apply -f https://github.com/triggermesh/scoby/releases/latest/download/scoby.yaml
```

Refer to [releases](https://github.com/triggermesh/scoby/releases) for previous versions.

## Development Version

Development version can be installed using [ko](https://github.com/ko-build/ko). Make sure that `triggermesh` namespace exists before running this command.

```console
ko apply -f ./config
```

## Namespaced Deployment

When Scoby is used in a reduced set of namespaces, permissions can be limited to only those namespaces.

Adapt the `ClusterRoleBindings` manifests contained in the [config folder](https://github.com/triggermesh/scoby/tree/main/config). For each namespace where Scoby must be supported, there must be a namespaced binding. Each `ClusterRoleBinding` must have a distinct name. Make sure to remove the non-namespaced binding since it should be no longer needed.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: scoby-controller-my-namespace
  namespace: my-namespace
  labels:
    app.kubernetes.io/part-of: triggermesh
...
```

This way we are scoping the `ClusterRoles` to those namespaces in use.

The controller needs to be informed and environment variable that contains a comma separated list of supported namespaces, an empty value meaning `all namespaces`. Edit the controller manifest and add the environment entry.

```yaml
        env:
        - name: SCOBY_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: WORKING_NAMESPACES
          value: my-namespace,your-namespace
```
