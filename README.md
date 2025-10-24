**Fork Notice:** 

This repo is a fork of the [Leaseweb's](https://github.com/leaseweb/cloudstack-csi-driver) maitained cloudstack-csi-driver, which is in-turn a fork of [Apalia's](https://github.com/apalia/cloudstack-csi-driver) cloudstack-csi-driver

# CloudStack CSI Driver

[![Go Reference](https://pkg.go.dev/badge/github.com/cloudstack/cloudstack-csi-driver.svg)](https://pkg.go.dev/github.com/cloudstack/cloudstack-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudstack/cloudstack-csi-driver)](https://goreportcard.com/report/github.com/cloudstack/cloudstack-csi-driver)
[![Release](https://github.com/cloudstack/cloudstack-csi-driver/workflows/Release/badge.svg?branch=main)](https://github.com/cloudstack/cloudstack-csi-driver/actions)

This repository provides a [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec)
plugin for [Apache CloudStack](https://cloudstack.apache.org/).

## Usage with Kubernetes

### Requirements

- Minimal Kubernetes version: v1.25

- The Kubernetes cluster must run in CloudStack. Tested only in a KVM zone.

- A disk offering with custom size must be available, with type "shared".

- In order to match the Kubernetes node and the CloudStack instance,
  they should both have the same name. If not, it is also possible to use
  [cloud-init instance metadata](https://cloudinit.readthedocs.io/en/latest/topics/instancedata.html)
  to get the instance name: if the node has cloud-init enabled, metadata will
  be available in `/run/cloud-init/instance-data.json`; you should then make
  sure that `/run/cloud-init/` is mounted from the node.

- Kubernetes nodes must be in the Root domain, and be created by the CloudStack
  account whose credentials are used in [configuration](#configuration).

### Configuration

Create the CloudStack configuration file `cloud-config`.

It should have the following format, defined for the [CloudStack Kubernetes Provider](https://github.com/apache/cloudstack-kubernetes-provider):

```ini
[Global]
api-url = <CloudStack API URL>
api-key = <CloudStack API Key>
secret-key = <CloudStack API Secret>
ssl-no-verify = <Disable SSL certificate validation: true or false (optional)>
```

Create a secret named `cloudstack-secret` in namespace `kube-system`:

```
kubectl create secret generic \
  --namespace kube-system \
  --from-file ./cloud-config \
  cloudstack-secret
```

If you have also deployed the [CloudStack Kubernetes Provider](https://github.com/apache/cloudstack-kubernetes-provider),
you may use the same secret for both tools.

### Deployment

```
kubectl apply -f https://github.com/cloudstack/cloudstack-csi-driver/releases/latest/download/manifest.yaml
```

### Creation of Storage classes

#### Manually

A storage class can be created manually: see [example](./examples/k8s/0-storageclass.yaml).

The `provisioner` value must be `csi.cloudstack.apache.org`.

The `volumeBindingMode` must be `WaitForFirstConsumer`, in order to delay the
binding and provisioning of a PersistentVolume until a Pod using the
PersistentVolumeClaim is created. It enables the provisioning of volumes
in respect to topology constraints (e.g. volume in the right zone).

The storage class must also have a parameter named
`csi.cloudstack.apache.org/disk-offering-id` whose value is the CloudStack disk
offering ID.

**Reclaim Policy**: Storage classes can have a `reclaimPolicy` of either `Delete` or `Retain`. If no `reclaimPolicy` is specified, it defaults to `Delete`. 

- `Delete`: When a PVC is deleted or a CKS cluster (Managed Kubernetes Cluster in CloudStack) is deleted, the associated persistent volumes and their underlying CloudStack disk volumes will be automatically removed.
- `Retain`: Persistent volumes and their underlying CloudStack disk volumes will be preserved even after PVC deletion or cluster deletion, allowing for manual recovery or data preservation.

#### Using cloudstack-csi-sc-syncer

The tool `cloudstack-csi-sc-syncer` may also be used to synchronize CloudStack
disk offerings to Kubernetes storage classes.

[More info...](./cmd/cloudstack-csi-sc-syncer/README.md)

> **Note:** The VolumeSnapshot CRDs (CustomResourceDefinitions) of version 8.3.0 are installed in this deployment. If you use a different version, please ensure compatibility with your Kubernetes cluster and CSI sidecars.


```
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.3.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.3.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.3.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml

```

### Usage

Example:

```
kubectl apply -f ./examples/k8s/pvc.yaml
kubectl apply -f ./examples/k8s/pod.yaml
```

## Building

To build the driver binary:

```
make build-cloudstack-csi-driver
```

To build the container images:

```
make container
```


## Volume Snapshots

**NOTE:** To create volume snapshots in KVM, make sure to set the `kvm.snapshot.enabled` global setting to true and restart the Management Server

### Volume snapshot creation
For Volume snapshots to be created, the following configurations need to be applied:

```
kubectl apply -f deploy/k8s/00-snapshot-crds.yaml        # Installs the VolumeSnapshotClass, VolumeSnapshotContent and VolumeSnapshtot CRDs
kubectl apply -f deploy/k8s/volume-snapshot-class.yaml   # Defines VolumeSnapshotClass for CloudStack CSI driver
```

Once the CRDs are installed, the snapshot can be taken by applying:
```
kubectl apply ./examples/k8s/snapshot/snapshot.yaml
```

In order to take the snapshot of a volume, `persistentVolumeClaimName` should be set to the right PVC name that is bound to the volume whose snapshot is to be taken.

You can check CloudStack volume snapshots if the snapshot was successfully created. If for any reason there was an issue, it can be investgated by checking the logs of the cloudstack-csi-controller pods: cloudstack-csi-controller, csi-snapshotter and snapshot-controller containers

```
kubectl logs -f <cloudstack-csi-controller pod_name> -n kube-system # defaults to tailing logs of cloudstack-csi-controller
kubectl logs -f <cloudstack-csi-controller pod_name> -n kube-system -c csi-snapshotter
kubectl logs -f <cloudstack-csi-controller pod_name> -n kube-system -c snapshot-controller
```

### Restoring a Volume snapshot

To restore a volume snapshot:
1. Restore a snapshot and Use it in a pod
* Create a PVC from the snapshot - for example ./examples/k8s/snapshot/pvc-from-snapshot.yaml
* Apply the configuration:
```
kubectl apply -f ./examples/k8s/snapshot/pvc-from-snapshot.yaml
```
* Create a pod that uses the restored PVC; example pod config ./examples/k8s/snapshot/restore-pod.yaml
```
kubectl apply -f ./examples/k8s/snapshot/restore-pod.yaml
```
2. To restore a snapshot when using a deployment
Update the deployment to point to the restored PVC

```
spec:
  volumes:
    - name: app-volume
      persistentVolumeClaim:
        claimName: pvc-from-snapshot
```


### Deletion of a volume snapshot

To delete a volume snapshot
One can simlpy delete the volume snapshot created in kubernetes using

```
kubectl delete volumesnapshot snapshot-1       # here, snapshot-1 is the name of the snapshot created
```

#### Troubleshooting issues with volume snapshot deletion
If for whatever reason, snapshot deletion gets stuck, one can troubleshoot the issue doing the following:

* Inspect the snapshot

```
kubectl get volumesnapshot <snapshot-name> [-n <namespace>] -o yaml
```

Look for the following section:
```
metadata:
  finalizers:
    - snapshot.storage.kubernetes.io/volumesnapshot-as-source
```

If finalizers are present, Kubernetes will not delete the resource until they are removed or resolved.

* Patch to Remove Finalizers

```
kubectl patch volumesnapshot <snapshot-name> [-n <namespace>] --type=merge -p '{"metadata":{"finalizers":[]}}'
```

**Caution:** This bypasses cleanup logic. Use only if you're certain the snapshot is no longer needed at the CSI/backend level

### What happens when you restore a volume from a snapshot
* The CSI external-provisioner (a container in the cloudstack-csi-controller pod) sees the new PVC and notices it references a snapshot
* The CSI driver's `CreateVolume` method is called with a `VolumeContentSource` that contains the snapshot ID
* The CSI driver creates a new volume from the snapshot (using the CloudStack's createVolume API)
* The new volume is now available as a PV (persistent volume) and is bound to the new PVC
* The volume is NOT attached to any node just by restoring from a snapshot, the volume is only attached to a node when a Pod that uses the new PVC is scheduled on a node
* The CSI driver's `ControllerPublishVolume` and `NodePublishVolume` methods are called to attach and mount the volume to the node where the Pod is running

Hence to debug any issues during restoring a snapshot, check the logs of the cloudstack-csi-controller, external-provisioner containers 

```
kubectl logs -f <cloudstack-csi-controller pod_name> -n kube-system # defaults to tailing logs of cloudstack-csi-controller
kubectl logs -f <cloudstack-csi-controller pod_name> -n kube-system -c external-provisioner
```

## Additional General Notes:

**Node Scheduling Best Practices**: When deploying applications that require specific node placement, use `nodeSelector` or `nodeAffinity` instead of `nodeName`. The `nodeName` field bypasses the Kubernetes scheduler, which can cause issues with storage provisioning. When a StorageClass has `volumeBindingMode: WaitForFirstConsumer`, the CSI controller relies on scheduler decisions to properly bind PVCs. Using `nodeName` prevents this scheduling integration, potentially causing PVC binding failures.

**Network CIDR Considerations**: When deploying CKS (CloudStack Kubernetes Service) clusters on pre-existing networks, avoid using the `10.0.0.0/16` CIDR range as it conflicts with Calico's default pod network configuration. This overlap can prevent proper CSI driver initialization and may cause networking issues within the cluster.

## See also

- [CloudStack Kubernetes Provider](https://github.com/apache/cloudstack-kubernetes-provider) - Kubernetes Cloud Controller Manager for Apache CloudStack
- [CloudStack documentation on storage](http://docs.cloudstack.apache.org/en/latest/adminguide/storage.html)
- [CSI (Container Storage Interface) specification](https://github.com/container-storage-interface/spec)

## History

Apalia SAS originally started the [CloudStack CSI Driver](https://github.com/apalia/cloudstack-csi-driver)
project, which was later [forked and progressed](https://github.com/apalia/cloudstack-csi-driver/forks)
by several members of the CloudStack community, notably [Leaseweb](https://github.com/leaseweb/cloudstack-csi-driver).

This repository attempts to widen the scope of the original project to make it work across
hypervisors (KVM, VMware, XenServer/XCP-ng) and add support for domains, projects, CKS, CAPC, and advanced storage
operations such as volume snapshots.

## License

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at <http://www.apache.org/licenses/LICENSE-2.0>

## Contributors

[![CloudStack CSI Driver Contributors](https://contrib.rocks/image?repo=cloudstack/cloudstack-csi-driver&anon=0&max=500)](https://github.com/cloudstack/cloudstack-csi-driver/graphs/contributors)
