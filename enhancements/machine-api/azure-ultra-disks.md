---
title: azure-ultra-disks
authors:
  - "@damdo"
reviewers:
  - "@JoelSpeed"
  - "@alexander-demichev"
  - "@elmiko"
approvers:
  - "@JoelSpeed"
  - "@elmiko"
api-approvers:
  - "@JoelSpeed"
creation-date: 2022-01-28
last-updated: 2022-02-11
tracking-link:
  - "https://issues.redhat.com/browse/OCPCLOUD-1377"
see-also:
replaces:
superseded-by:
---

# Azure Ultra Disks support in Machine API

## Summary

Enable OCP users to leverage [Azure Ultra Disks](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disks) on Machine API provisioned hosts on Azure via Data Disks or to attach them to Machines via Persistent Volumes (PVs).

## Motivation

Allow users running OCP clusters on Azure to leverage Ultra Disk Storage to achieve high performance on disks to meet their specific storage performance requirements.

### Goals

- Provide automation for creating and attaching Azure Ultra Disks as Data Disks to Machines
- Enable attaching Azure Ultra Disks via Persistent Volumes (PVs) to Machines

### Non-Goals
- Providing support for specifying Disk throughput (DiskMBpsReadWrite) or Disk IOPS (DiskIOPSReadWrite) for Azure Ultra Disk as Data Disks
  - This is not supported upstream nor by the Azure Go SDK and thus it is not part of the goals of this enhancement. This will instead be supported in Persistent Volumes as these fields are supported by the Azure CSI driver
  - These values will automatically be set by Azure based on parameters optimized according to the size chosen for the Ultra Disk
  - As a workaround this can be manually updated in the Azure Portal on the disk settings after disk creation without having to detach the disk from the instance
- Any logic for providing support for BYO (Bring Your Own) already existing Azure Ultra Disk / Data Disk and attach it to a new Machine
  - This is not supported upstream and thus it is not part of the goals of this enhancement
- Any logic for providing support for other high performance disk types from the same or other providers
- Any support for the feature for Azure Stack Hub (ASH). Ultra Disks are currently not supported there

## Proposal

The following requirements for integration will be necessary for adding Ultra Disks support:
1. Required configuration for supporting Data Disk should be added to the `AzureMachineProviderSpec`.
This will allow users to set Ultra Disks as Data Disks for the Machine. This feature has been made available upstream in Cluster API Provider for Azure (CAPZ) by: https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/1478
1. Required configuration for specifying the ability in `AzureMachineProviderSpec` to allow the Azure CSI driver to provision Ultra Disks as PVs for the Machine. See the issue tracking the implementation of this feature in the upstream Cluster API Provider for Azure (CAPZ): https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1852

### User Stories

**Story 1**
As a developer, I'd like to have the ability to use Azure Ultra Disks as Data Disks for my Machines.

**Story 2**
As a developer, I'd like to specify some basic specs (Disk Size, Logical Unit Number (LUN), Caching Type) for creating Azure Ultra Disks as Data Disks for my Machines.

**Story 3**
As a developer, I'd like to have the ability to create Persistent Volume Claims (PVCs) which can dynamically bind to a StorageClass backed by Azure Ultra Disk and to mount them successfully to Pods for my workloads needs.

### API Extensions

#### Extension for Ultra Disks as Data Disks

The API design is taken from the upstream Cluster API Provider for Azure (CAPZ), [here](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/fee74172efc0db07a244ceb870be759db56e0f83/api/v1beta1/azuremachine_types.go#L77-L79) and [here](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/fee74172efc0db07a244ceb870be759db56e0f83/api/v1beta1/types.go#L465-L483).
The upstream design is followed as closely as possible to ensure smoother future migrations between MAPI and CAPI.

A new field will be added to the `AzureMachineProviderSpec` struct `DataDisks`. The interface will be a slice of `DataDisk` structs allowing multiple Data Disks and various fields specific to them to be further specified.

`DataDisk` will need a slightly modified version of the `ManagedDiskParameters` type (due to an extra `UltraSSD_LRS` possible value) for its `ManagedDisk` field.

For this reason a new Composite type field `DataDiskManagedDiskParameters` will be added and the original `ManagedDiskParameters`, used in `OSDisk`, will be renamed to `OSDiskManagedDiskParameters`.

`DataDisk` will also need a `DeletionPolicy` field to instruct on how the Data Disk should be treated at machine deletion. In other words, it will describe whether to retain or delete the data disk when the machine is deleted from the cloud provider. This field will be marked as required in order to make it very clear with the user what the fate of the data disk is going to be.

A new `StorageAccountType` type that describes the possible values for the `StorageAccountType` field on `DataDisk` will be added.

A new `CachingTypeOption` type that describes the possible values for the `CachingType` field on `DataDisk` will also be added.

A new `DiskDeletionPolicyType` type that describes the possible deletion policy types for the `DeletionPolicy` field on `DataDisk` will also be added.

```go
type AzureMachineProviderSpec struct {
  // Existing fields will not be modified
  ...

  // DataDisk specifies the parameters that are used to add one or more data disks to the machine.
  // +optional
  DataDisks []DataDisk `json:"dataDisks,omitempty"`
}

// DataDisk specifies the parameters that are used to add one or more data disks to the machine.
// A Data Disk is a managed disk that's attached to a virtual machine to store application data.
// It differs from an OS Disk as it doesn't come with a pre-installed OS, and it cannot contain the boot volume.
// It is registered as SCSI drive and labeled with the chosen `lun`. e.g. for `lun: 0` the raw disk device will be available at `/dev/disk/azure/scsi1/lun0`.
//
// As the Data Disk disk device is attached raw to the virtual machine, it will need to be partitioned, formatted with a filesystem and mounted, in order for it to be usable.
// This can be done by creating a custom userdata Secret with custom Ignition configuration to achieve the desired initialization.
// At this stage the previously defined `lun` is to be used as the "device" key for referencing the raw disk device to be initialized.
// Once the custom userdata Secret has been created, it can be referenced in the Machine's `.providerSpec.userDataSecret`.
// For further guidance and examples, please refer to the official OpenShift docs.
type DataDisk struct {
	// NameSuffix is the suffix to be appended to the machine name to generate the disk name.
	// Each disk name will be in format <machineName>_<nameSuffix>.
	// NameSuffix name must start and finish with an alphanumeric character and can only contain letters, numbers, underscores, periods or hyphens.
	// The overall disk name must not exceed 80 chars in length.
	// +kubebuilder:validation:Pattern:=`^[a-zA-Z0-9](?:[\w\.-]*[a-zA-Z0-9])?$`
	// +kubebuilder:validation:MaxLength:=78
	// +kubebuilder:validation:Required
	NameSuffix string `json:"nameSuffix"`
	// DiskSizeGB is the size in GB to assign to the data disk.
	// +kubebuilder:validation:Minimum=4
	// +kubebuilder:validation:Required
	DiskSizeGB int32 `json:"diskSizeGB"`
	// ManagedDisk specifies the Managed Disk parameters for the data disk.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is a ManagedDisk with with storageAccountType: "Premium_LRS" and diskEncryptionSet.id: "Default".
	// +optional
	ManagedDisk DataDiskManagedDiskParameters `json:"managedDisk,omitempty"`
	// Lun Specifies the logical unit number of the data disk.
	// This value is used to identify data disks within the VM and therefore must be unique for each data disk attached to a VM.
	// This value is also needed for referencing the data disks devices within userdata to perform disk initialization through Ignition (e.g. partition/format/mount).
	// The value must be between 0 and 63.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=63
	// +kubebuilder:validation:Required
	Lun int32 `json:"lun,omitempty"`
	// CachingType specifies the caching requirements.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is CachingTypeNone.
	// +optional
	// +kubebuilder:validation:Enum=None;ReadOnly;ReadWrite
	CachingType CachingTypeOption `json:"cachingType,omitempty"`
	// DeletionPolicy specifies the data disk deletion policy upon Machine deletion.
	// Possible values are "Delete","Detach".
	// When "Delete" is used the data disk is deleted when the Machine is deleted.
	// When "Detach" is used the data disk is detached from the Machine and retained when the Machine is deleted.
	// +kubebuilder:validation:Enum=Delete;Detach
	// +kubebuilder:validation:Required
	DeletionPolicy DiskDeletionPolicyType `json:"deletionPolicy"`
}

// DiskDeletionPolicyType defines the possible values for DeletionPolicy.
type DiskDeletionPolicyType string

// These are the valid DiskDeletionPolicyType values.
const (
	// DiskDeletionPolicyTypeDelete means the DiskDeletionPolicyType is "Delete".
	DiskDeletionPolicyTypeDelete DiskDeletionPolicyType = "Delete"
	// DiskDeletionPolicyTypeDetach means the DiskDeletionPolicyType is "Detach".
	DiskDeletionPolicyTypeDetach DiskDeletionPolicyType = "Detach"
)

// CachingTypeOption defines the different values for a CachingType.
type CachingTypeOption string

// These are the valid CachingTypeOption values.
const (
	// CachingTypeReadOnly means the CachingType is "ReadOnly".
	CachingTypeReadOnly CachingTypeOption = "ReadOnly"
	// CachingTypeReadWrite means the CachingType is "ReadWrite".
	CachingTypeReadWrite CachingTypeOption = "ReadWrite"
	// CachingTypeNone means the CachingType is "None".
	CachingTypeNone CachingTypeOption = "None"
)

// DataDiskManagedDiskParameters is the parameters of a DataDisk managed disk.
type DataDiskManagedDiskParameters struct {
	// StorageAccountType is the storage account type to use.
	// Possible values include "Standard_LRS", "Premium_LRS" and "UltraSSD_LRS".
	// +kubebuilder:validation:Enum=Standard_LRS;Premium_LRS;UltraSSD_LRS
	StorageAccountType StorageAccountType `json:"storageAccountType"`
	// DiskEncryptionSet is the disk encryption set properties.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is a DiskEncryptionSet with id: "Default".
	// +optional
	DiskEncryptionSet *DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
}

// StorageAccountType defines the different storage types to use for a ManagedDisk.
type StorageAccountType string

// These are the valid StorageAccountType types.
const (
	// "StorageAccountStandardLRS" means the Standard_LRS storage type.
	StorageAccountStandardLRS StorageAccountType = "Standard_LRS"
	// "StorageAccountPremiumLRS" means the Premium_LRS storage type.
	StorageAccountPremiumLRS StorageAccountType = "Premium_LRS"
	// "StorageAccountUltraSSDLRS" means the UltraSSD_LRS storage type.
	StorageAccountUltraSSDLRS StorageAccountType = "UltraSSD_LRS"
)
```

#### Extension for Ultra Disks as Persistent Volumes
To allow the mounting of Persistent Volume Claims of StorageClass `UltraSSD_LRS` to Pods living on an arbitrary Node backed by a MAPI Machine, there is a need for the ability to attach an Ultra Disk to the Azure instance backing that Machine.

To give instances this ability an Azure _Additional Capability_ (`AdditionalCapabilities.UltraSSDEnabled`) must be specified on the instance. This will allow/disallow the attachment of Ultra Disks to it.

Furthermore the `UltraSSDEnabled` Azure _Additional Capability_ must be present on instances attaching Ultra Disks as Data Disks and the plan in that scenario is to pilot its toggling automatically when an Ultra Disk is specified as a Data Disk.

So when coming up with a proposal for extending the API, the fact that the `UltraSSDEnabled` _Additional Capability_ has the ability to govern both features and how it must change depending on what features the user wants to use, must both be taken into account.

For this purpose a new field will be added to the 
`AzureMachineProviderSpec` struct. The interface will be an optional `UltraSSDCapability` struct of type `AzureUltraSSDCapabilityState` which will allow specifying an Enum of states to instruct the provider whether to enable the capability for attaching Ultra Disks to an instance or not:
- if `UltraSSDCapability` is `Enabled` then the `UltraSSDEnabled` Additional capability will be set to `true` for the Machine, which will allow Ultra Disks attachments in both ways (via Data Disks and via PVCs)
- if `UltraSSDCapability` is `Disabled` then the `UltraSSDEnabled` Additional capability will be set to `false` for the Machine, which will disallow Ultra Disks attachements in both ways  (via Data Disks and via PVCs)
- if `UltraSSDCapability` is omitted, the provider may enable the `UltraSSDEnabled` capability depending on whether any Ultra Disks are specified as Data Disks

```go
type AzureMachineProviderSpec struct {
  // Existing fields will not be modified
  ...

  // UltraSSDCapability enables or disables Azure UltraSSD capability for a virtual machine.
  // When omitted, the platform may enable the capability based on the configuration of data disks.
  // +kubebuilder:validation:Enum:="Enabled";"Disabled"
  // +optional
  UltraSSDCapability AzureUltraSSDCapabilityState `json:"ultraSSDCapability,omitempty"`
}

// AzureUltraSSDCapabilityState defines the different states of an UltraSSDCapability
type AzureUltraSSDCapabilityState string

// These are the valid AzureUltraSSDCapabilityState states.
const (
  // "AzureUltraSSDCapabilityEnabled" means the Azure UltraSSDCapability is Enabled
  AzureUltraSSDCapabilityEnabled AzureUltraSSDCapabilityState = "Enabled"
  // "AzureUltraSSDCapabilityDisabled" means the Azure UltraSSDCapability is Disabled
  AzureUltraSSDCapabilityDisabled AzureUltraSSDCapabilityState = "Disabled"
)
```

### Implementation Details/Notes/Constraints

#### Initializing Data Disks 
While one of the goals for this enhancement is to provide automation for creating and attaching Azure Ultra Disks as Data Disks to Machines, there is one caveat.
Once an Azure instance is created and a Data Disk (in this case of type Ultra Disk) is attached to said instance, the Disk will be seen as a _raw_ block device by the OS.

This means that in order for the Data Disk to be usable by workloads it will require some form of initialization, which, given the high performing nature of Ultra Disks (and more generally of Data Disks) is best suited to be left to the cluster administrator.
This way they will be able to decide how to best slice the disk in partitions, while also choosing how to format them and autonomously deciding where to have them mounted.

For this reason, the following example is provided as guidance on how a cluster administrator can specify flexible configurations for their Data Disks:

The following steps assume the Machine about to be launched to which the raw Data Disk will be attached, will be running RHEL CoreOS (RHCOS) or Fedora CoreOS (FCOS) and that [CoreOS Ignition](https://github.com/coreos/ignition) is set up and already performing first boot install and configuration:
1. _Custom user-data Secret creation_:

    In order to leverage CoreOS Ignition, which allows manipulation of disks during initramfs, the administrator needs to create a custom user-data configuration (in this example stored in a Secret object) where a desired extension of the Ignition configuration can be specified:
      - Create a new secret from the `worker-user-data` Secret
        - Export the userData section of the Secret to a text file:
          ```sh
          $ oc -n openshift-machine-api get secret worker-user-data --template='{{index .data.userData | base64decode}}' | jq > userData.txt
          ```
        - Edit the text file to add the storage, filesystems, and systemd stanzas for the partitions you want to use for the new node. The user can specify any Ignition configuration parameters as needed. For example if a Data Disk with `lun: 0` is planned to be added for the Machine, the Ingition config to partition, filesystem format and mount a device on `lun0` can be specified like so. Please note:
          - if the `lun` specified on the Data Disk field on the Machine differs, `*lun0*` in this example will need to change accordingly. 
          - `sizeMiB` and `startMiB` will need to change depending on the size of the disk and the desired partition size.
          - `format` will need to change depending on the desired filesystem format (provided it is supported by Ignition).
          - systemd unit mount `name` and `contents` will need to change to mirror the `path` and `device` values chosen in the `filesystems` section.
          - further paritions, filesystems and mounts can be added to the configuration provided they are compatible with the Ignition spec and version that the OS observes.
  
          Edit the `userData.txt`, and right after the `"ignition":{ ... }` block (and before the last closing bracket `}`) add comma `,`, then add these following blocks:
          ```json
          "storage": {
            "disks": [
              {
                "device": "/dev/disk/azure/scsi1/lun0",
                "partitions": [
                  {
                    "label": "lun0p1",
                    "sizeMiB": 1024,
                    "startMiB": 0
                  }
                ]
              }
            ],
            "filesystems": [
              {
                "device": "/dev/disk/by-partlabel/lun0p1",
                "format": "xfs",
                "path": "/var/lib/lun0p1"
              }
            ]
          },
          "systemd": {
            "units": [
              {
                "contents": "[Unit]\nBefore=local-fs.target\n[Mount]\nWhere=/var/lib/lun0p1\nWhat=/dev/disk/by-partlabel/lun0p1\nOptions=defaults,pquota\n[Install]\nWantedBy=local-fs.target\n",
                "enabled": true,
                "name": "var-lib-lun0p1.mount"
              }
            ]
          }
          ```
        - Extract the `disableTemplating` section from the `worker-user-data` Secret to a text file:
          ```sh
          $ oc -n openshift-machine-api get secret worker-user-data --template='{{index .data.disableTemplating | base64decode}}' | jq > disableTemplating.txt
          ```
        - Create the new user data secret file from the two text files. This user data Secret passes the additional node partition information in the userData.txt file to the newly created node.
          ```sh
          $ oc -n openshift-machine-api create secret generic worker-user-data-x5 --from-file=userData=userData.txt --from-file=disableTemplating=disableTemplating.txt
          ```
1. _MachineSet with Data Disk and custom user-data creation_:

    - Create a new MachineSet YAML where:
      - A new Data Disk is defined under `dataDisks` and the `name` under `userDataSecret` reference the name of the newly created user data Secret:
        ```yaml
        dataDisks:
        - nameSuffix: ultrassd
          lun: 0
          diskSizeGB: 4
          cachingType: None
          deletionPolicy: Detach
          managedDisk:
            storageAccountType: UltraSSD_LRS
        userDataSecret:
          name: worker-user-data-x5
        ```
      - A label for identifiying Nodes originated from Machines spawned by this MachineSet is set by adding under `.spec.template.spec.metadata`:
        ```yaml
        labels:
          disktype: ultrassd
        ```
    - When a Machine belonging to the new MachineSet is `Running` and a Node is created and linked to it, the result of partitioning can be verified with:
      ```sh
      $ oc debug node/<node-name> -- chroot /host lsblk
      ```
1. _Using of Data Disks with workloads_:

    The Data Disk is now available on the host Node at the desired mount point (in this example: `/var/lib/lun0p1`). Finally, to use it with workloads, for example in a Pod, the host path can be mounted via an `hostPath` share in a privileged Pod, like so:
    ```yaml
    kind: Pod
    apiVersion: v1
    metadata:
      name: ssd-benchmark
    spec:
      containers:
      - name: ssd-benchmark
        image: quay.io/ddonati/ssd-benchmark:1.1.7
        securityContext:
          privileged: true
        command:
          - "/bin/sh"
          - "-c"
          - "--"
        args: ["while true; do /usr/bin/ssd-benchmark; sleep 30; done"]
        volumeMounts:
        - name: lun0p1
          mountPath: "/tmp"
      volumes:
        - name: lun0p1
          hostPath:
            path: /var/lib/lun0p1
            type: Directory
      nodeSelector:
        disktype: ultrassd
    ```

### Risks and Mitigations

N/A

## Design Details

### Open Questions

The ability to attach Ultra Disks to Pods (and Machines) via Persistent Volumes Claims [is not yet supported](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1852) in the upstream Cluster API Provider for Azure (CAPZ).

I ([@damdo](https://github.com/damdo)) volunteered to propose an implementation of this feature in the upstream Cluster API Provider for Azure (CAPZ).
But there is a question on whether the to-be-proposed upstream API Extension for Ultra Disks as Persistent Volumes (which will likely mirror what it's being proposed here) ends up differing in the final accepted design and implementation from the one proposed for this downstream forum (OCP).

### Test Plan
TBD 

### Graduation Criteria
The addition of API fields to Machine API implies that the feature is GA from the beginning, no graduation criteria are required.

#### Dev Preview -> Tech Preview

N/A

#### Tech Preview -> GA

N/A

#### Removing a deprecated feature

No features will be removed as a part of this proposal.

### Upgrade / Downgrade Strategy

Existing clusters being upgraded will not have any undesired effect as this these features do not interfere with any other one.

Once configured, on downgrade, the Machine API components will not know about the new fields, and as such, they will ignore the interface type field if specified. The usage of an Ultra Disk will not affect removal of Machines after a downgrade, there should be no detrimental effect of a downgrade on Machine API.

Machines created with the new interface type will be unaffected and will persist within the cluster until an administrator decides to remove them.

If a Machine created with the new interface type is manually removed after the downgrade and it has one or more Data Disks attached to it, they will need to be manually cleaned-up by the administrator after the Machine has been deleted.

### Version Skew Strategy

N/A

### Operational Aspects of API Extensions

#### Failure Modes

1) If for any reason the Ultra Disks as Data Disks feature is failing to work users won't be able to attach Ultra Disks as Data Disks and new Machines that are meant to be created with such disks attached may go into a `Failed` state
1) If for any reason the Ultra Disks as Persistent Volumes feature is failing to work, users won't be able to mount volumes created via Persistent Volume Claims that are backed by the `UltraSSD_LRS` StorageClass. Workloads are likely to fail starting-up and will be stuck in `ContainerCreating` state waiting for Volumes to becoume available
1) If the region, availability zone or instance size chosen for the Machine are not in the [matrix of scenarios where Ultra Disks are supported](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-types#ultra-disk-limitations), the creation of the Machine will fail and the Machine will go into `Failed` state
1) If a Persistent Volume Claim (PVC) backed by an `UltraSSD_LRS` StorageClass is mounted into a Pod and the Pod starts on a Node where the Machine backing it lacks the `UltraSSDEnabled` additional capability, this will result in the Pod getting stuck in `ContainerCreating` as the Ultra Disk will not be able to get attached to the Machine

#### Support Procedures

- If the creation and attachment of Ultra Disks as Data Disks is not working as expected, because of conflicting configuration around the`UltraSSDEnabled` additional capability (e.g. `UltraSSDEnabled` set to `Disabled` but an UltraSSD is specified within `DataDisks`) the Machine will not provisioned and will go in `Failed` state with the following Event:
  - `FailedCreate  NmNNs (xN over NmNNs)  azure-controller  InvalidConfiguration: failed to reconcile machine "xyz": failed to create vm xyz:
failure sending request for machine xyz: cannot create vm: compute.VirtualMachinesClient#CreateOrUpdate:
Failure sending request: StatusCode=400
-- Original Error: Code="InvalidParameter" Message="StorageAccountType UltraSSD_LRS can be used only when additionalCapabilities.ultraSSDEnabled is set."
Target="managedDisk.storageAccountType`
- If the deletion of Ultra Disks as Data Disks is not working as expected, Machines will be deleted and the Data Disks will be orphaned. This will be visible in error logs of the provider with a message along the lines of: `failed to delete Data Disk: xyz`
- If the creation and attachment of Ultra Disks as Data Disks is not working as expected because the user has chosen a region, availability zone or instance size which are incompatible with Ultra Disks, the machine provisioning will fail.
  This will be visibile in the error logs of the provider with a message along the lines of: `vm size xyz does not support ultra disks in location xyz. select a different vm size or disable ultra disks`
- If the mounting of an Ultra Disk backed Persistent Volume Claim (PVC), as a Volume in a Pod, is not working and the the Pod is stuck in `ContainerCreating` mode the issue can be debugged by describing the Pod.
  An example of this could be an error caused by the absence of the `UltraSSDEnabled` additional capability on the Machine backing the Node that is hosting the aformentioned Pod. This will manifest with the following Event:
  - `Warning  FailedAttachVolume  NNs (xN over NmNNs)  attachdetach-controller  AttachVolume.Attach failed for volume "pvc-nnn" : rpc error: code = Unknown desc = Attach volume "/subscriptions/nnn/resourceGroups/nnn/providers/Microsoft.Compute/disks/pvc-nnn" to instance "nnnn" failed with Retriable: false, RetryAfter: 0s, HTTPStatusCode: 400, RawError: {
  "error": {
    "code": "InvalidParameter",
    "message": "StorageAccountType UltraSSD_LRS can be used only when additionalCapabilities.ultraSSDEnabled is set.",
    "target": "managedDisk.storageAccountType"
  }
}`
  - An alert will also be triggered on this condition.

## Implementation History

N/A

## Drawbacks

N/A

## Alternatives

The alternative to adding support for Ultra Disks on MAPI is to reject the RFE. In this case, the end user must use some method outside of OpenShift to attach Ultra Disks to the instances of their clusters. This prevents the user from leveraging this type of high performance disk with Machine API.

### Future implementation

In the future, we may wish to extend the capabilities of Ultra Disks as Data Disks to allow specifying settings like Disk throughput and Disk IOPS.

This will need to be supported in the Azure Go SDK first. 

This is currently considered to be out of scope within this enhancement.
