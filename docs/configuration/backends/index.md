title: Overview

# Backends

DNS4ACME supports multiple storage backends, depending on its build-time configuration. You can use any of these backends to store update key and DNS record information.

| Backend                     | Description                                                                                                                                                                           |
|-----------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Kubernetes](kubernetes.md) | Stores information using a [CustomResourceDefinition](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) in the Kubernetes API server. |