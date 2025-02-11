---
title: pin-and-pre-load-images
authors:
- "@jhernand"
reviewers:
- "@avishayt"   # To ensure that this will be usable with the appliance.
- "@danielerez" # To ensure that this will be usable with the appliance.
- "@mrunalp"    # To ensure that this can be implemented with CRI-O and MCO.
- "@nmagnezi"   # To ensure that this will be usable with the appliance.
- "@oourfali"   # To ensure that this will be usable with the appliance.
approvers:
- "@sinnykumari"
- "@mrunalp"
api-approvers:
- "@sdodson"
- "@zaneb"
- "@deads2k"
- "@JoelSpeed"
creation-date: 2023-09-21
last-updated: 2023-09-21
tracking-link:
- https://issues.redhat.com/browse/RFE-4482
see-also:
- https://github.com/openshift/enhancements/pull/1432
- https://github.com/openshift/api/pull/1609
replaces: []
superseded-by: []
---

# Pin and pre-load images

## Summary

Provide an mechanism to pin and pre-load container images.

## Motivation

Slow and/or unreliable connections to the image registry servers interfere with
operations that require pulling images. For example, an upgrade may require
pulling more than one hundred images. Failures to pull those images cause
retries that interfere with the upgrade process and may eventually make it
fail. One way to improve that is to pull the images in advance, before they are
actually needed, and ensure that they aren't removed. Doing that provides a
more consistent upgrade time in those environments. That is important when
scheduling upgrades into maintenance windows, even if the upgrade might not
otherwise fail.

### User Stories

#### Pre-load and pin upgrade images

As the administrator of a cluster that has a low bandwidth and/or unreliable
connection to an image registry server I want to pin and pre-load all the
images required for the upgrade in advance, so that when I decide to actually
perform the upgrade there will be no need to contact that slow and/or
unreliable registry server and the upgrade will successfully complete in a
predictable time.

#### Pre-load and pin application images

As the administrator of a cluster that has a low bandwidth and/or unreliable
connection to an image registry server I want to pin and pre-load the images
required by my application in advance, so that when I decide to actually deploy
it there will be no need to contact that slow and/or unreliable registry server
and my application will successfully deploy in a predictable time.

### Goals

Provide a mechanism that cluster administrators can use to pin and pre-load
container images.

### Non-Goals

We wish to use the mechanism described in this enhancement to orchestrate
upgrades without a registry server. But that orchestration is not a goal
of this enhancement; it will be part of a separate enhancement based on
parts of https://github.com/openshift/enhancements/pull/1432.

## Proposal

### Workflow Description

1. The administrator of a cluster uses the new `PinnedImageSet` custom resource
to request that a set of container images are pinned and pre-loaded:

    ```yaml
    apiVersion: machineconfiguration.openshift.io/v1alpha1
    kind: PinnedImageSet
    metadata:
      name: my-pinned-images
    spec:
      nodeSelector:
        matchLabels:
          node-role.kubernetes.io/control-plane: ""
      pinnedImages:
      - quay.io/openshift-release-dev/ocp-release@sha256:...
      - quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
      - quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
      ...
    ```

1. A new `PinnedImageSetController` sub-controller of the
_machine-config-controller_ ensures that all the images are pinned and
pulled in all the nodes that match the node selector.

### API Extensions

A new `PinnedImageSet` custom resource definition will be added to the
`machineconfiguration.openshift.io` API group.

The new custom resource definition is described in detail in
https://github.com/openshift/api/pull/1609.

### Implementation Details/Notes/Constraints

Starting with version 4.14 of OpenShift CRI-O will have the capability to pin
certain images (see [this](https://github.com/cri-o/cri-o/pull/6862) pull
request for details). That capability will be used to pin all the images
required for the upgrade, so that they aren't garbage collected by kubelet and
CRI-O.

In addition when the CRI-O service is upgraded and restarted it removes all the
images. This used to be done by the `crio-wipe` service, but is now done
internally by CRI-O. It can be avoided setting the `version_file_persist`
configuration parameter to "", but that would affect all images, not just the
pinned ones. This behavior needs to be changed in CRI-O so that pinned images
aren't removed, regardless of the value of `version_file_persist`.

The changes to pin the images will be done in a file inside the
`/etc/crio/crio.conf.d` directory. To avoid potential conflicts with files
manually created by the administrator the name of this file will be the name of
the `PinnedImageSet` custom resource concatenated with the UUID assigned by the
API server. For example, if the custom resource is this:

```yaml
apiVersion: machineconfiguration.openshift.io/v1alpha1
kind: PinnedImageSet
metadata:
  name: my-pinned-images
  uuid: 550a1d88-2976-4447-9fc7-b65e457a7f42
spec:
  pinnedImages:
  - quay.io/openshift-release-dev/ocp-release@sha256:...
  - quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
  - quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
  ...
```

Then the complete path will be this:

```txt
/etc/crio/crio.conf.d/my-pinned-images-550a1d88-2976-4447-9fc7-b65e457a7f42.conf
```

The content of the file will the `pinned_images` parameter containing the list
of images:

```toml
pinned_images=[
  "quay.io/openshift-release-dev/ocp-release@sha256:...",
  "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...",
  "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...",
  ...
]
```

In addition to the list of images to be pinned, the `PinnedImageSet` custom
resource will also contain a node selector. This is intended to support
different sets of images for different kinds of nodes. For example, to pin
different images for control plane and worker nodes the user could create two
`PinnedImageSet` custom resources:

```yaml
# For control plane nodes:
apiVersion: machineconfiguration.openshift.io/v1alpha1
kind: PinnedImageSet
metadata:
  name: my-control-plane-pinned-images
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/control-plane: ""
  pinnedImages:
  ...

---

# For worker nodes:
apiVersion: machineconfiguration.openshift.io/v1alpha1
kind: PinnedImageSet
metadata:
  name: my-worker-pinned-images
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/worker: ""
  pinnedImages:
  ...
```

This is specially convenient for pinning images for upgrades: there are many
images that are needed only by the control plane nodes and there is no need to
have them consuming disk space in worker nodes.

When no node selector is specified the images will be pinned in all the nodes
of the cluster.

The _machine-config-controller_ will grow a new `PinnedImageSetController` sub
controller that will watch the `PinnedImageSet` custom resources. When one of
those is created, updated or deleted the controller will start a daemon set that
will do the following in each node of the cluster:

1. Verify that there is enough disk space available in the machine to pull the
   images.

1. Create, modify or delete the CRI-O pinning configuration file.

1. Reload the CRI-O configuration, with the equivalent of `systemctl reload crio`.

    Note that currently CRI-O doesn't recalculate the set of pinned images on
    reload, support for that will need to be added.

1. Use the CRI-O gRPC API to run the equivalent of `crictl pull` for each of the
images.

This needs to be a daemon set running in each node of the cluster because it
needs to create configuration files on the node, and use the CRI-O gRPC API,
which is only accessible via `localhost`.

This logic could be part of a new daemon set, specific for this, or else part of
the _machine-config-daemon_.

In order to verify that there is enough disk space we will first check which of
the images in the `PinnedImageSet` have not yet been downloaded, using CRI-O
gRPC API. For those that haven't been download we will then fetch from the
registry server the size of the blobs of the layers (only the size, not the
actual data).

Calculating the exact total size of the layers once uncompressed and written to
disk is difficult without deep knowledge of the underplaying containers storage
library. Instead of that we will assume that the disk space required is twice
the size of the blobs of the layers. That is an heuristic that works well for
complete OpenShift releases: blobs take 16 GiB and when they are written to disk
they take 32 GiB.

If the calculate disk space exceeds the available disk space then the
`PinnedImageSet` will be marked as failed and the process will not move forward.
For example:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
  - type: Failed
    status: "True"
    message: |
      Pulling the images in `node12` requires at least 16 GiB of disk
      space, but there are only 10 GiB available
```

Note that even with this check it will still be possible (but less likely) to
have failures to pull images due to disk space: the heuristic could be wrong,
and there may be other components pulling images or consuming disk space in
some other way. Those failures will be detected and reported in the status of
the `PinnedImageSet` when CRI-O fails to pull the image.

The steps above will happen in all the nodes of the cluster. The daemon set
will be provided with enough information to ensure that each pod applies only
the changes required for the node where it runs, according to the node
selectors in the `PinnedImageSet` custom resources.

When all the images have been successfully pinned and pulled in all the matching
nodes the `PinnedImageSetController` will set the `Ready` condition to `True`:

```yaml
status:
  conditions:
  - type: Ready
    status: "True"
  - type: Failed
    status: "False"
```

If something fails the `Failed` condition will be set to `True`, and the
details of the error will be in the message. For example if `node12` fails to
pull an image:

```yaml
status:
  pinnedImages:
  - quay.io/openshift-release-dev/ocp-release@sha256:...
  - quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
  conditions:
  - type: Ready
    status: "False"
  - type: Failed
    status: "True"
    message: Node 'node12' failed to pull image `quay.io/...` because ...
```

We will consider using the new _machine-config-operator_ state reporting
mechanism introduced [https://github.com/openshift/api/pull/1596](here) to
report additional details about the progress or issues of each node.

Additional information may be needed inside the `status` field of the
`PinnedImageSet` custom resources in order to implement the mechanisms described
above. For example, it may be necessary to have conditions per node, something
like this:

```yaml
status:
  node0:
  - type: Ready
    status: "False"
  - type: Failed
    status: "True"
  node1:
  - type: Ready
    status: "True"
  - type: Failed
    status: "True"
  ...
```

Those details are explicitly left out of this document, because they are mostly
implementation details, and not relevant for the user of the API.

### Risks and Mitigations

Pre-loading disk images can consume a large amount of disk space. For example,
pre-loading all the images required for an upgrade can consume more 32 GiB.
This is a risk because disk exhaustion can affect other workloads and the
control plane. There is already a mechanism to mitigate that: the kubelet
garbage collection. To ensure disk space issues are reported and that garbage
collection doesn't interfere we will introduced new mechanisms:

1. Disk space will be checked before trying to pre-load images, and if there are
issues they will be reported explicitly via the status of the `PinnedImageSet`
status.

1. If disk space is exhausted while pulling the images, then the issue will be
reported via the status of the `PinnedImageSet` as well. Typically these issues
are reported as failures to pull images in the status of pods, but in this case
there are no pods pulling the images. CRI-O will still report these issues in
the log (if there is space for that), like for any other image pull.

1. If disk exhaustion happens after the images have been pulled, then we will
ensure that the kubelet garbage collector doesn't select these images. That
will be handled by the image pinning support in CRI-O: even if kubelet asks
CRI-O to delete a pinned image CRI-O will not delete it.

1. The recovery steps in the documentation will be amended to ensure that these
images aren't deleted to recover disk space.

### Drawbacks

This approach requires non trivial changes to the machine config operator.

## Design Details

### Open Questions

None.

### Test Plan

We add a CI test that verifies that images are correctly pinned and pre-loaded.

### Graduation Criteria

The feature will ideally be introduced as `Dev Preview` in OpenShift 4.X,
moved to `Tech Preview` in 4.X+1 and declared `GA` in 4.X+2.

#### Dev Preview -> Tech Preview

- Availability of the CI test.

- Obtain positive feedback from at least one customer.

#### Tech Preview -> GA

- User facing documentation created in
[https://github.com/openshift/openshift-docs](openshift-docs).

#### Removing a deprecated feature

Not applicable, no feature will be removed.

### Upgrade / Downgrade Strategy

Upgrades from versions that don't support the `PinnedImageSet` custom resource
don't require any special handling because the custom resource is optional:
there will be no such custom resource in the upgraded cluster.

Downgrades to versions that don't support the `PinnedImageSet` custom resource
don't require any changes. The existing pinned images will be ignored in the
downgraded cluster, and will eventually be garbage collected.

### Version Skew Strategy

Not applicable.

### Operational Aspects of API Extensions

#### Failure Modes

Image pulling may fail due to lack of disk space or other reasons. This will be
reported via the conditions in the `PinnedImageSet` custom resource. See the
risks and mitigations section for details.

#### Support Procedures

Nothing.

## Implementation History

There is an initial prototype exploring some of the implementation details
described here in this [https://github.com/jhernand/upgrade-tool](repository).

## Alternatives

The alternative to this is to manually pull the images in all the nodes of the
cluster, manually create the `/etc/crio/crio.conf.d/pin.conf` file and manually
reload the CRI-O service.

## Infrastructure Needed

Infrastructure will be needed to run the CI test described in the test plan
above.
