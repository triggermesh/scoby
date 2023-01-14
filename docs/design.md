# Scoby Design

## Kubernetes Controllers

Scoby manage component registrations dynamically. Components registrations can be created, modified and deleted.
Each registration creates new CRDs and start a controller for them using `controller-runtime`.

- Registration modifications are not supported yet by Scoby, but they will follow the CRD versioning pattern.
- Registration deletions are not supported yet by `controller-runtime` but there is an open [issue](https://github.com/kubernetes-sigs/controller-runtime/issues/1884) and [PR](https://github.com/kubernetes-sigs/controller-runtime/pull/2099) that we need to track.
