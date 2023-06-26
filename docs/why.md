# Why Create Scoby

At TriggerMesh we enable event driven architectures (EDA) at Kubernetes providing a range of components that you can use to store, process, consume and produce events.

The consuming and producing components are called sources and targets and can be found in the [triggermesh/triggermesh](https://github.com/triggermesh/triggermesh) repository. There are many of them and the list will most likely grow. However, the likelyhood that a given project needs to use all of these components simultaneously is low.

## The Problems

Let's dive into the problems that Scoby solves in the case of TriggerMesh.

### Too Many Controllers

For each TriggerMesh component that runs on Kubernetes, a [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) exists. Users are enabled to create TriggerMesh instances and expect a controller to act on it and perform a number of operations that can be summarized into some controller logic that uses the created instance information, and the management of a given generated Kubernetes workload, either a deployment or a Knative service.

We maintain a controller per CRD, and although we abstract away a good part of the controller and reconciling code, it is still mandatory to take care of it.

### Painful Customization

All TriggerMesh component controllers are bundled into a single controller. The controller does eager loading of all its internal object informers and thus requires that all CRDs have been created. What that means from the user perspective is that you need to install about 50 CRDs even if you plan to use only 1 of our components, and inside the controller process, 50 reconcilers will be created.

Some people created their own controller bundles that only import the controllers that they needed, but that is not the user experience we want to offer at TriggerMesh.

### No Easy Externsiblity

Often we are asked how to extend TriggerMesh with custom components. While creating a piece of software that consumes or emmits [CloudEvents](https://cloudevents.io/) using HTTP is quite simple, creating a reusable component that can be instanciated by any user at Kubernetes is a more demanding task.

### Knative Dependency

We have heard from users that want to use TriggerMesh at vanilla Kubernetes scenarios. TriggerMesh controllers decide how components workloads are rendered, and for any of those exposing an endpoint a Knative service is used.

This requires that users install Knative Serving, even if the components that they plan to use is not rendered as a Knative service.

## Scoby

Scoby solves the problems above, and also bring some limitations to the picture:

- We no longer need to write controllers, as long as Scoby can be instructed to create the workload that we expect. Currently Kubernetes deployments (with optional services) and Knative services are supported.
- Scoby only creates controllers for the CRDs that are registered. Users can now register and unregister the components that they choose.
- Creating new components is not limited to the TriggerMesh team. Package your component in a container, create the CRD that your users will instantiate and register using Scoby. This means your users get the same developer experience regardless of whether the component is provided by TriggerMesh or custom.
- Whether using Knative or not is up to users to decide. A Scoby registration can be modified to select which form factor should be rendered for each component; if Knative is not present, select Kubernetes deployment + Kubernetes service.

### Limitations

While Scoby simplifies and brings flexibility to TriggerMesh components it bears some limitations:

- Some controllers execute custom logic to transform a user's Kubernetes object into the workload's environment variables that Scoby does not support.
- A controller might also need to call external system to manage external resources.
- Some controllers might update custom fields at the user object's status.
- Some controllers need to create or customize resources like ServiceAccounts.

This limitations will be tackled in Scoby when it can be solved in a generic way, or using [hooks](reference/hooks.md) under specific scenarios.
