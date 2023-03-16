# Development

## Run From Code

To setup a development environment:

```console
# Apply registration
kubectl apply -f config/scoby.triggermesh.io_crdregistrations.yaml

# Run controller
go run cmd/controller/main.go
```

## License Headers

License headers must to be written using SPDX identifier

```go
// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0
```

Use [addlicense](https://github.com/google/addlicense) to automatically add the header to all go files.

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```
