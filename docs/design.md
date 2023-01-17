# Scoby Design

## Kubernetes Controllers

Scoby manage component registrations dynamically.

There are 2 types of registrations:

- CRD based: a registration includes information about a CRD that scoby will add a controller for. This allows for finer control of parameters.
- Scoby based: each registration includes information to create a new CRD and start a controller

- Registration modifications are not supported yet by Scoby, but they will follow the CRD versioning pattern.
- Registration deletions are not supported yet by `controller-runtime` but there is an open [issue](https://github.com/kubernetes-sigs/controller-runtime/issues/1884) and [PR](https://github.com/kubernetes-sigs/controller-runtime/pull/2099) that we need to track.

## CRD Based

1. Users register a new `CRDRegistration` object with information about an existing CRD.
2. The registration controller builds a controller for the CRD and creates an abstract representation of the CRD.
3. Upon each instance of the CRD the registration form factor is used to render the kubernetes objects, and the abstract representation of the CRD is matched with the instance and converted into environment variables.