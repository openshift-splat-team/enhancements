---
title: vsphere-multi-account-credential-management
authors:
  - "@rvanderp"
reviewers:
  - "@jcpowermac, for vSphere platform expertise, please review privilege definitions and vCenter integration"
  - "@patrickdillon, for installer team review, please review installation workflow changes"
  - "@joelspeed, for cloud expertise, please review credential management approach"
  - "@gnufied, for storage team review, please review CSI driver credential separation"
approvers:
  - TBD
api-approvers:
  - TBD
creation-date: 2026-01-28
last-updated: 2026-08-11
status: provisional
tracking-link:
  - TBD
see-also:
  - "/enhancements/installer/vsphere-ipi.md"
  - "/enhancements/cloud-integration/cloud-credential-operator.md"
replaces: []
superseded-by: []
---

# vSphere Multi-Account Credential Management

## Summary

This enhancement proposes support for administrator-provisioned, component-specific vCenter credentials for OpenShift on vSphere. Rather than having the cloud-credential-operator (CCO) create vCenter accounts (which would require administrative privileges), this enhancement enables administrators to pre-provision separate vCenter service accounts for each OpenShift component and provide those credentials to OpenShift. CCO distributes the credentials to the appropriate components, while the vsphere-problem-detector validates that credentials have sufficient privileges. This approach supports enterprise security requirements where account provisioning is controlled by infrastructure teams, while still achieving least-privilege isolation between components.

## Motivation

OpenShift on vSphere currently uses a single vCenter account for all operations across all components (installer, machine-api-operator, CSI driver, diagnostics). This creates several problems:

1. **Excessive Privilege Exposure:** Every component has access to all privileges, even those it doesn't need
2. **Large Blast Radius:** A compromised credential in any component grants full cluster and potentially vSphere infrastructure access
3. **Poor Auditability:** vCenter audit logs cannot distinguish which OpenShift component performed an action
4. **Compliance Challenges:** Single-account architecture conflicts with SOC2 separation of duties and PCI-DSS least privilege requirements
5. **Credential Rotation Complexity:** Rotating credentials requires updating all components simultaneously

Analysis of OpenShift source code across seven repositories (installer, machine-api-operator, vmware-vsphere-csi-driver, cluster-storage-operator, govmomi, vsphere-problem-detector, cloud-credential-operator) reveals that each component requires distinct privilege subsets:

| Component | Required Privileges | Current State |
|-----------|---------------------|---------------|
| Installer | ~45 (full set) | Uses shared credentials |
| Machine API | ~35 (VM lifecycle) | Uses shared credentials |
| CSI Driver | ~10-15 (storage) | Uses shared credentials |
| Cloud Controller | ~10 (read-only) | Uses shared credentials |
| Diagnostics | ~5 (read-only) | Uses shared credentials |

### Why Administrator-Provisioned (Not CCO-Minted)

Having CCO automatically create vCenter accounts would require:
- Administrative privileges on vCenter (Global.Licenses, Admin role)
- Access to vCenter SSO or identity source management
- Elevated trust in OpenShift to manage vCenter accounts

This conflicts with enterprise security practices where:
- Account provisioning is controlled by dedicated infrastructure/security teams
- Service accounts go through approval workflows
- Account creation is audited separately from account usage
- Identity management is centralized (Active Directory, LDAP)

**This enhancement takes the administrator-provisioned approach:** Administrators create the accounts using their existing processes, then provide the credentials to OpenShift.

### User Stories

* As a **security-conscious cluster administrator**, I want to provide each OpenShift component with separate vCenter credentials that I've pre-provisioned, so that I maintain control over account creation while achieving least-privilege isolation.

* As a **compliance officer**, I want OpenShift to support separation of duties for vCenter access using accounts provisioned through our standard identity management processes, so that our OpenShift deployment meets SOC2 and PCI-DSS requirements.

* As a **vSphere administrator**, I want to create vCenter service accounts with specific privileges for each OpenShift component, following my organization's account provisioning procedures, and then configure OpenShift to use these accounts.

* As a **platform operator**, I want OpenShift to validate that each component's credentials have the required privileges, so that I can catch configuration errors during deployment rather than at runtime.

* As a **security team member**, I want documentation of exactly what privileges each OpenShift component requires, so that I can create appropriately-scoped vCenter roles and accounts before cluster deployment.

* As a **day-2 operations engineer**, I want to rotate credentials for individual OpenShift components independently by updating secrets, so that credential rotation follows my organization's rotation policies without affecting other components.

### Goals

1. **Support per-component credential configuration:** Enable administrators to provide distinct vCenter credentials for each component (machine-api, CSI driver, diagnostics).

2. **Document precise privilege requirements:** Provide authoritative documentation of privileges required by each component, organized by vSphere object scope.

3. **Enable independent credential rotation:** Support updating credentials for one component without affecting others.

4. **Maintain backward compatibility:** Continue supporting single shared credential (passthrough mode) for simpler deployments.

### Non-Goals

1. **Automatic vCenter account creation:** CCO will NOT create vCenter accounts; this requires admin privileges the operator should not have.

2. **Direct identity source integration:** CCO will not integrate with AD/LDAP to create accounts.

3. **Privilege escalation or modification:** CCO will not modify roles or add privileges to existing accounts.

4. **External secret manager integration:** Integration with HashiCorp Vault, CyberArk, etc. is out of scope.

5. **Runtime privilege discovery:** Automatic detection of what privileges a component actually uses is not in scope.

6. **Provide tooling for role/account creation:** Offering scripts and documentation for creating vCenter roles and accounts with correct privileges is outside the scope of this enhancement.

## Proposal

### Overview

This enhancement extends OpenShift to support administrator-provisioned, per-component vCenter credentials with multi-vCenter support:

1. **Credential storage:** Credentials are stored in install-config.yaml or a hidden file in the user's home directory (`~/.vsphere/credentials`)
2. **Multi-vCenter support:** Each component can have separate credentials for each vCenter in a multi-vCenter topology
3. **CredentialsRequest specifications:** Define precise privilege requirements per component
4. **Graceful degradation:** Fall back to shared credentials if per-component credentials not provided

### Component Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────────┐
│                       Administrator Workflow (Per vCenter)                       │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│  1. Review privilege      2. Create vCenter       3. Create vCenter              │
│     documentation            roles (govc/UI)         accounts                    │
│         │                        │                       │                        │
│         ▼                        ▼                       ▼                        │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐            │
│  │ Privilege Docs  │     │ govc role.create│     │ govc sso.user   │            │
│  │ per component   │     │ PowerCLI        │     │ AD/LDAP         │            │
│  └─────────────────┘     └─────────────────┘     └─────────────────┘            │
│         │                        │                       │                        │
│         └────────────────────────┴───────────────────────┘                        │
│                                  │                                                │
│                    ┌─────────────┴─────────────┐                                 │
│                    ▼                           ▼                                 │
│           ┌─────────────────┐         ┌─────────────────┐                        │
│           │   vCenter 1     │         │   vCenter 2     │                        │
│           │   (roles +      │         │   (roles +      │                        │
│           │    accounts)    │         │    accounts)    │                        │
│           └─────────────────┘         └─────────────────┘                        │
│                                  │                                                │
│                                  ▼                                                │
│                    4. Store credentials in:                                       │
│                       - install-config.yaml, OR                                   │
│                       - ~/.vsphere/credentials                                    │
│                                  │                                                │
└──────────────────────────────────┼────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          OpenShift Cluster                                       │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐  │
│  │                    cloud-credential-operator                               │  │
│  ├───────────────────────────────────────────────────────────────────────────┤  │
│  │                                                                             │  │
│  │  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐        │  │
│  │  │CredentialsRequest│   │ Multi-vCenter   │    │ Credential      │        │  │
│  │  │  Controller      │──▶│ Privilege       │──▶│ Distributor     │        │  │
│  │  │                  │   │ Validator       │    │                 │        │  │
│  │  └─────────────────┘    └─────────────────┘    └─────────────────┘        │  │
│  │                                                        │                    │  │
│  └────────────────────────────────────────────────────────┼────────────────────┘  │
│                                                           │                        │
│      ┌────────────────────────────────────────────────────┼────────────────────┐  │
│      │                                                    │                    │  │
│      ▼                                                    ▼                    ▼  │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐              │
│  │ machine-api     │    │ csi-driver      │    │ diagnostics     │              │
│  │ credentials     │    │ credentials     │    │ credentials     │              │
│  │ (all vCenters)  │    │ (all vCenters)  │    │ (all vCenters)  │              │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘              │
│                                                                                   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Workflow Description

**Actors:**
- **vSphere administrator:** Creates vCenter accounts and roles
- **cluster administrator:** Provides credentials to OpenShift, manages cluster
- **cloud-credential-operator:** Validates and distributes credentials
- **OpenShift components:** Consume credentials for vCenter operations

#### Pre-Installation Workflow

1. The vSphere administrator reviews the privilege documentation for each component
2. The vSphere administrator creates custom roles in vCenter using provided scripts:
   ```bash
   # Example: Create machine-api role
   govc role.create openshift-machine-api \
       Sessions.ValidateSession \
       VirtualMachine.Config.AddNewDisk \
       VirtualMachine.Interact.PowerOn \
       # ... (all required privileges)
   ```
3. The vSphere administrator creates service accounts for each component:
   ```bash
   # Using vCenter SSO
   govc sso.user.create -p 'SecurePassword123!' ocp-cluster1-machine-api
   govc sso.user.create -p 'SecurePassword456!' ocp-cluster1-csi-driver
   ```
4. The vSphere administrator assigns roles to accounts on appropriate vSphere objects:
   ```bash
   govc permissions.set -principal 'ocp-cluster1-machine-api@vsphere.local' \
       -role openshift-machine-api \
       -propagate=true \
       /Datacenter/vm/openshift-cluster1
   ```
5. The vSphere administrator provides credentials to the cluster administrator

#### Installation Workflow (Per-Component Credentials Mode)

Credentials can be provided in two ways:

**Option A: install-config.yaml (Recommended for automation)**

The cluster administrator includes per-component credentials directly in install-config.yaml:

```yaml
apiVersion: v1
baseDomain: example.com
metadata:
  name: my-cluster
platform:
  vsphere:
    vcenters:
      - server: vcenter1.example.com
        user: ocp-installer@vsphere.local
        password: <installer-password>
        datacenters:
          - DC1
        componentCredentials:
          machineAPI:
            user: ocp-machine-api@vsphere.local
            password: <machine-api-password>
          csiDriver:
            user: ocp-csi@vsphere.local
            password: <csi-password>
          cloudController:
            user: ocp-ccm@vsphere.local
            password: <ccm-password>
          diagnostics:
            user: ocp-diagnostics@vsphere.local
            password: <diagnostics-password>
      - server: vcenter2.example.com
        user: ocp-installer@vsphere.local
        password: <installer-password-vc2>
        datacenters:
          - DC2
        componentCredentials:
          machineAPI:
            user: ocp-machine-api@vsphere.local
            password: <machine-api-password-vc2>
          csiDriver:
            user: ocp-csi@vsphere.local
            password: <csi-password-vc2>
          cloudController:
            user: ocp-ccm@vsphere.local
            password: <ccm-password-vc2>
          diagnostics:
            user: ocp-diagnostics@vsphere.local
            password: <diagnostics-password-vc2>
    failureDomains:
      - name: zone-a
        server: vcenter1.example.com
        # ...
      - name: zone-b
        server: vcenter2.example.com
        # ...
```

**Option B: Hidden credentials file (Recommended for interactive use)**

The cluster administrator creates a credentials file at `~/.vsphere/credentials`:

```yaml
# ~/.vsphere/credentials
# Supports per-vCenter, per-component credentials

vcenter1.example.com:
  # Default credentials (used by installer)
  user: ocp-installer@vsphere.local
  password: <installer-password>

  # Per-component credentials
  componentCredentials:
    machineAPI:
      user: ocp-machine-api@vsphere.local
      password: <machine-api-password>
    csiDriver:
      user: ocp-csi@vsphere.local
      password: <csi-password>
    cloudController:
      user: ocp-ccm@vsphere.local
      password: <ccm-password>
    diagnostics:
      user: ocp-diagnostics@vsphere.local
      password: <diagnostics-password>

vcenter2.example.com:
  user: ocp-installer@vsphere.local
  password: <installer-password-vc2>
  componentCredentials:
    machineAPI:
      user: ocp-machine-api@vsphere.local
      password: <machine-api-password-vc2>
    csiDriver:
      user: ocp-csi@vsphere.local
      password: <csi-password-vc2>
    cloudController:
      user: ocp-ccm@vsphere.local
      password: <ccm-password-vc2>
    diagnostics:
      user: ocp-diagnostics@vsphere.local
      password: <diagnostics-password-vc2>
```

The installer reads from `~/.vsphere/credentials` when:
- Credentials are not specified in install-config.yaml
- The `VSPHERE_CREDENTIALS_FILE` environment variable points to a custom location

**Installation Process:**

1. The installer reads credentials from install-config.yaml or ~/.vsphere/credentials.
2. For each vCenter entry that includes `componentCredentials`, the installer:
   a. Creates a named Secret per component in the `openshift-config` namespace. Secret names follow the convention `vsphere-creds-<component>` (e.g., `vsphere-creds-machine-api`, `vsphere-creds-csi-driver`).
   b. Each Secret contains keys in the format `<vcenter-server>.username` / `<vcenter-server>.password` for every vCenter in the topology.
   c. The Secrets are owned by the `Infrastructure` resource (`metadata.ownerReferences`) so that cluster lifecycle operations (e.g., destroy) clean them up.
3. The installer sets `spec.platformSpec.vsphere.credentialsMode` to `PerComponent` on the `Infrastructure/cluster` resource and populates the `componentCredentials` field with `SecretReference` entries pointing to each created Secret (name + namespace).
4. CCO reconciles the `Infrastructure` resource, reads each referenced Secret, and verifies connectivity to each vCenter (see [Credential Distribution Workflow](#credential-distribution-workflow)).
5. If connectivity verification fails, CCO reports the failure via `CredentialsProvisionFailed` conditions on the relevant `CredentialsRequest`.
6. On successful verification, CCO copies credentials into target-namespace Secrets consumed by each component (preserving the existing cluster-storage-operator → CVO → CSI operator cloud-credentials contract where applicable).
7. The `vsphere-problem-detector` independently validates that each component's credentials have the required privileges for their operations.
8. Components start using their designated credentials.

**Secret Lifecycle:**

- **Ownership:** The `openshift-config` Secrets are owned by the `Infrastructure/cluster` resource. Component-namespace copies (e.g., `openshift-machine-api/vsphere-cloud-credentials`) are owned by the corresponding `CredentialsRequest`.
- **Update:** When an administrator updates a Secret in `openshift-config`, CCO detects the change via a watch and propagates the updated credentials to component namespaces.
- **Rotation:** Administrators rotate credentials by updating the `openshift-config` Secret with new values. CCO distributes without component restart; components pick up new credentials on the next vCenter session reconnect. The vsphere-problem-detector validates that updated credentials have sufficient privileges.
- **Rollback:** If updated credentials fail connectivity verification, CCO retains the last-known-good credentials in the component namespace and sets a `CredentialsProvisionFailed` condition. The administrator can revert the `openshift-config` Secret to restore the previous credentials.

#### Alternative: Post-Installation Configuration

For existing clusters migrating to per-component credentials:

1. The cluster administrator creates per-component secrets with multi-vCenter support:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: vsphere-creds-machine-api
     namespace: openshift-config
   type: Opaque
   stringData:
     # Credentials for each vCenter
     vcenter1.example.com.username: "ocp-machine-api@vsphere.local"
     vcenter1.example.com.password: "<password-vc1>"
     vcenter2.example.com.username: "ocp-machine-api@vsphere.local"
     vcenter2.example.com.password: "<password-vc2>"
   ```
2. The cluster administrator updates the infrastructure configuration:
   ```yaml
   apiVersion: config.openshift.io/v1
   kind: Infrastructure
   metadata:
     name: cluster
   spec:
     platformSpec:
       vsphere:
         credentialsMode: PerComponent
         componentCredentials:
           machineAPI:
             secretRef:
               name: vsphere-creds-machine-api
               namespace: openshift-config
           csiDriver:
             secretRef:
               name: vsphere-creds-csi-driver
               namespace: openshift-config
           cloudController:
             secretRef:
               name: vsphere-creds-cloud-controller
               namespace: openshift-config
           diagnostics:
             secretRef:
               name: vsphere-creds-diagnostics
               namespace: openshift-config
   ```
3. CCO reconciles the new configuration and distributes credentials to components

#### Credential Distribution Workflow

When CCO receives credentials for a component (per vCenter):

0. CCO reads referenced Secrets exclusively from the `openshift-config` namespace. If a `SecretReference` points to any other namespace, CCO rejects it immediately and sets `CredentialsProvisionFailed` without attempting to read the Secret.
1. For each vCenter configured in the cluster:
   a. CCO extracts the component credentials for that vCenter
   b. CCO connects to the vCenter using the provided credentials to verify connectivity
2. If connectivity is confirmed on all vCenters, CCO provisions the credential to the component
3. If connectivity fails on any vCenter, CCO:
   - Sets condition `CredentialsProvisionFailed` on the CredentialsRequest
   - Logs detailed message identifying which vCenter(s) failed authentication

**Note:** Privilege validation is **not** performed by CCO. The `vsphere-problem-detector` is responsible for checking that credentials have sufficient privileges for their intended operations. CCO's role is limited to credential distribution and basic connectivity verification.

### API Extensions

#### install-config.yaml Extension (Installer API)

The installer API is extended to support per-component credentials within each vCenter definition:

```go
// VCenter stores the vCenter connection fields and per-component credentials.
// This is part of the installer's install-config API.
type VCenter struct {
    // Server is the FQDN or IP address of the vCenter server.
    Server string `json:"server"`

    // Port is the TCP port that will be used to connect to vCenter.
    // +optional
    Port int32 `json:"port,omitempty"`

    // User is the username to use when connecting to vCenter.
    User string `json:"user"`

    // Password is the password for the user.
    Password string `json:"password"`

    // Datacenters is the list of datacenters to use within this vCenter.
    Datacenters []string `json:"datacenters"`

    // ComponentCredentials specifies per-component credentials for this vCenter.
    // If not specified, the main User/Password is used for all components.
    // +optional
    ComponentCredentials *VCenterComponentCredentials `json:"componentCredentials,omitempty"`
}

// VCenterComponentCredentials defines per-component credentials for a single vCenter.
type VCenterComponentCredentials struct {
    // MachineAPI specifies credentials for machine-api-operator on this vCenter.
    // +optional
    MachineAPI *VCenterCredential `json:"machineAPI,omitempty"`

    // CSIDriver specifies credentials for the vSphere CSI driver on this vCenter.
    // +optional
    CSIDriver *VCenterCredential `json:"csiDriver,omitempty"`

    // CloudController specifies credentials for the cloud controller manager on this vCenter.
    // +optional
    CloudController *VCenterCredential `json:"cloudController,omitempty"`

    // Diagnostics specifies credentials for vsphere-problem-detector on this vCenter.
    // +optional
    Diagnostics *VCenterCredential `json:"diagnostics,omitempty"`
}

// VCenterCredential stores username and password for a vCenter account.
type VCenterCredential struct {
    // User is the username for the account.
    User string `json:"user"`

    // Password is the password for the account.
    Password string `json:"password"`
}
```

#### Canonical vCenter Credential Key Format

Per-component Secrets store credentials keyed by vCenter server. Because Kubernetes Secret keys must conform to the DNS subdomain rules (alphanumeric, `-`, `_`, `.`), and raw IPv6 addresses contain colons (`:`) which are **invalid** in Secret keys, a normalization scheme is required.

**Key derivation rules:**

| VCenter.Server value | Port | Canonical key prefix |
|----------------------|------|----------------------|
| `vcenter.example.com` | (default 443) | `vcenter.example.com` |
| `vcenter.example.com` | 8443 | `vcenter.example.com-8443` |
| `192.168.1.100` | (default 443) | `192.168.1.100` |
| `192.168.1.100` | 8443 | `192.168.1.100-8443` |
| `fd00::1` | (default 443) | `fd00-0000-0000-0000-0000-0000-0000-0001` |
| `[fd00::1]` | 8443 | `fd00-0000-0000-0000-0000-0000-0000-0001-8443` |

**Normalization algorithm:**

1. Strip surrounding brackets from IPv6 addresses (e.g., `[fd00::1]` → `fd00::1`).
2. If the address is IPv6, expand to the full 8-group representation and replace every `:` with `-` (e.g., `fd00::1` → `fd00-0000-0000-0000-0000-0000-0000-0001`).
3. FQDNs and IPv4 addresses are lowercased and used verbatim.
4. If Port is specified and is not the default (443), append `-<port>` (e.g., `vcenter.example.com-8443`).
5. Append `.username` or `.password` to form the final Secret data key.

**Examples of resulting Secret keys:**

```yaml
stringData:
  # FQDN (default port)
  vcenter.example.com.username: "user@vsphere.local"
  vcenter.example.com.password: "secret"
  # IPv4 with non-default port
  192.168.1.100-8443.username: "user@vsphere.local"
  192.168.1.100-8443.password: "secret"
  # IPv6 (expanded, colons replaced with dashes)
  fd00-0000-0000-0000-0000-0000-0000-0001.username: "user@vsphere.local"
  fd00-0000-0000-0000-0000-0000-0000-0001.password: "secret"
```

The installer and CCO both apply the same normalization when reading and writing Secret keys. The `~/.vsphere/credentials` file entries use the original `VCenter.Server` value as the top-level YAML key (e.g., `fd00::1`); the installer normalizes when creating Secrets.

#### ~/.vsphere/credentials File Format

The credentials file follows a YAML format with per-vCenter entries, consistent with install-config.yaml:

```yaml
# ~/.vsphere/credentials
# File permissions should be 0600 (readable only by owner)

# Each top-level key is a vCenter server FQDN or IP
vcenter1.example.com:
  # Default credentials (used by installer and as fallback)
  user: admin-user@vsphere.local
  password: secret-password

  # Per-component credentials (optional)
  componentCredentials:
    machineAPI:
      user: ocp-machine-api@vsphere.local
      password: machine-api-password
    csiDriver:
      user: ocp-csi@vsphere.local
      password: csi-password
    cloudController:
      user: ocp-ccm@vsphere.local
      password: ccm-password
    diagnostics:
      user: ocp-diagnostics@vsphere.local
      password: diagnostics-password

vcenter2.example.com:
  user: admin-user@vsphere.local
  password: secret-password-vc2
  componentCredentials:
    machineAPI:
      user: ocp-machine-api@vsphere.local
      password: machine-api-password-vc2
    # ... other components
```

The installer reads credentials in this order of precedence:
1. Explicit credentials in install-config.yaml
2. `VSPHERE_CREDENTIALS_FILE` environment variable path
3. `~/.vsphere/credentials` default location

#### VSpherePlatformSpec Extension (Infrastructure API)

```go
// VSpherePlatformSpec holds configuration for the vSphere platform.
type VSpherePlatformSpec struct {
    // ... existing fields ...

    // CredentialsMode specifies how vSphere credentials are managed.
    // Valid values are:
    //   "Passthrough" - Single credential used for all components (default)
    //   "PerComponent" - Separate credentials per component
    // +optional
    CredentialsMode VSphereCredentialsMode `json:"credentialsMode,omitempty"`

    // ComponentCredentials specifies per-component credential references.
    // Only used when CredentialsMode is "PerComponent".
    // Each secret contains credentials for all vCenters.
    // +optional
    ComponentCredentials *VSphereComponentCredentials `json:"componentCredentials,omitempty"`
}

// VSphereCredentialsMode defines how credentials are managed.
// +kubebuilder:validation:Enum=Passthrough;PerComponent
type VSphereCredentialsMode string

const (
    // VSphereCredentialsModePassthrough uses single shared credentials.
    VSphereCredentialsModePassthrough VSphereCredentialsMode = "Passthrough"
    // VSphereCredentialsModePerComponent uses separate credentials per component.
    VSphereCredentialsModePerComponent VSphereCredentialsMode = "PerComponent"
)

// VSphereComponentCredentials defines credential references for each component.
type VSphereComponentCredentials struct {
    // MachineAPI specifies credentials for machine-api-operator.
    // The referenced secret contains keys in the format:
    //   <vcenter-server>.username and <vcenter-server>.password
    // +optional
    MachineAPI *SecretReference `json:"machineAPI,omitempty"`

    // CSIDriver specifies credentials for the vSphere CSI driver.
    // +optional
    CSIDriver *SecretReference `json:"csiDriver,omitempty"`

    // CloudController specifies credentials for the cloud controller manager.
    // +optional
    CloudController *SecretReference `json:"cloudController,omitempty"`

    // Diagnostics specifies credentials for vsphere-problem-detector.
    // +optional
    Diagnostics *SecretReference `json:"diagnostics,omitempty"`
}

// SecretReference identifies a secret in a namespace.
// Namespace is restricted to "openshift-config" to limit the blast radius of
// credential storage and simplify RBAC for CCO.
type SecretReference struct {
    // Name is the name of the secret.
    Name string `json:"name"`
    // Namespace is the namespace of the secret.
    // +kubebuilder:validation:Enum=openshift-config
    // Must be "openshift-config". CCO rejects references to other namespaces
    // during reconciliation and sets CredentialsProvisionFailed.
    Namespace string `json:"namespace"`
}
```

#### Per-Component Secret Format (Multi-vCenter)

Each component secret contains credentials for each vCenter, keyed by vCenter server FQDN:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-creds-machine-api
  namespace: openshift-config
type: Opaque
stringData:
  # Format: <vcenter-fqdn>.username and <vcenter-fqdn>.password
  vcenter1.example.com.username: "ocp-machine-api@vsphere.local"
  vcenter1.example.com.password: "password-for-vc1"
  vcenter2.example.com.username: "ocp-machine-api@vsphere.local"
  vcenter2.example.com.password: "password-for-vc2"
```

This format supports:
- **Different identity sources per vCenter:** Each vCenter can use different SSO domains or identity providers
- **Different account names:** While not recommended, different usernames can be used per vCenter
- **Independent password rotation:** Passwords can be rotated on one vCenter without affecting others
- **Flexible deployment:** Easily add or remove vCenters from the topology

#### Extended VSphereProviderSpec for CredentialsRequest

```go
// VSphereProviderSpec contains the privilege requirements for a component.
// Used by CCO to validate that provided credentials have sufficient privileges.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VSphereProviderSpec struct {
    metav1.TypeMeta `json:",inline"`

    // Permissions contains the list of permission sets required by this component.
    // CCO validates that the provided credentials have these privileges.
    Permissions []VSpherePermission `json:"permissions"`
}

// VSpherePermission defines a set of privileges for a specific vSphere object scope.
type VSpherePermission struct {
    // Privileges is the list of vCenter privilege IDs required.
    // Example: "VirtualMachine.Config.AddNewDisk", "Datastore.AllocateSpace"
    Privileges []string `json:"privileges"`

    // Scope specifies where these privileges are required.
    Scope VSpherePermissionScope `json:"scope"`

    // Propagate indicates whether privileges must propagate to child objects.
    // +optional
    Propagate bool `json:"propagate,omitempty"`
}

// VSpherePermissionScope defines the vSphere object(s) where privileges are checked.
type VSpherePermissionScope struct {
    // Type specifies the type of vSphere object.
    // Valid values: "vCenter", "Datacenter", "Cluster", "ResourcePool",
    //               "Folder", "Datastore", "Network"
    // +kubebuilder:validation:Enum=vCenter;Datacenter;Cluster;ResourcePool;Folder;Datastore;Network
    Type string `json:"type"`

    // VCenter identifies which vCenter this scope applies to, using the
    // canonical key derived from VCenter.Server (see "Canonical vCenter
    // Credential Key Format"). When empty, the scope applies to every
    // vCenter in the cluster topology.
    // +optional
    VCenter string `json:"vCenter,omitempty"`

    // Path is the concrete vSphere inventory path of the target object
    // (e.g., "/Datacenter/vm/openshift-cluster1"). When InferFromClusterConfig
    // is true this field is ignored; CCO resolves paths from the cluster's
    // failure domain configuration instead.
    // +optional
    Path string `json:"path,omitempty"`

    // InferFromClusterConfig indicates the path should be derived from
    // the cluster's infrastructure configuration (failure domains).
    // When true, CCO resolves every applicable target object from the
    // Infrastructure resource's failure domains and validates privileges
    // on each resolved object independently.
    // +optional
    InferFromClusterConfig bool `json:"inferFromClusterConfig,omitempty"`
}
```

**Privilege validation behavior (vsphere-problem-detector):**

- When `Propagate` is `true` in a `VSpherePermission`, the vsphere-problem-detector does **not** pass `Propagate` to `FetchUserPrivilegeOnEntities` (which does not accept it). Instead, it validates privileges on representative child objects (e.g., for a Folder scope with propagation, it checks both the folder and a child VM or subfolder) or inspects the permission assignment on the parent to confirm `propagate=true` is set.
- When `InferFromClusterConfig` is `true`, the vsphere-problem-detector iterates all failure domains in the `Infrastructure` resource, resolves each to the relevant vSphere object (datacenter, cluster, folder, datastore, network), and validates privileges on every resolved target.

### Privilege Requirements by Component

Based on comprehensive code analysis of OpenShift repositories:

#### Machine API Operator

**vCenter Root (no propagation):**
```text
Sessions.ValidateSession
InventoryService.Tagging.AttachTag
InventoryService.Tagging.CreateTag
InventoryService.Tagging.EditTag
InventoryService.Tagging.DeleteTag
```

**Cluster (with propagation):**
```text
Resource.AssignVMToPool
VApp.AssignResourcePool
```

**VM Folder (with propagation):**
```text
VirtualMachine.Config.AddExistingDisk
VirtualMachine.Config.AddNewDisk
VirtualMachine.Config.AddRemoveDevice
VirtualMachine.Config.AdvancedConfig
VirtualMachine.Config.Annotation
VirtualMachine.Config.CPUCount
VirtualMachine.Config.DiskExtend
VirtualMachine.Config.EditDevice
VirtualMachine.Config.Memory
VirtualMachine.Config.RemoveDisk
VirtualMachine.Config.Rename
VirtualMachine.Config.ResetGuestInfo
VirtualMachine.Config.Resource
VirtualMachine.Config.Settings
VirtualMachine.Interact.GuestControl
VirtualMachine.Interact.PowerOff
VirtualMachine.Interact.PowerOn
VirtualMachine.Interact.Reset
VirtualMachine.Inventory.Create
VirtualMachine.Inventory.CreateFromExisting
VirtualMachine.Inventory.Delete
VirtualMachine.Provisioning.Clone
VirtualMachine.Provisioning.DeployTemplate
VirtualMachine.State.CreateSnapshot
VirtualMachine.State.RemoveSnapshot
InventoryService.Tagging.ObjectAttachable
```

**Datastore (no propagation):**
```text
Datastore.AllocateSpace
Datastore.Browse
Datastore.FileManagement
```

**Network (no propagation):**
```text
Network.Assign
```

**Total: ~35 privileges**

#### CSI Driver

**vCenter Root (no propagation):**
```text
Cns.Searchable
StorageProfile.View
Sessions.ValidateSession
```

**VM Folder (with propagation):**
```text
VirtualMachine.Config.AddExistingDisk
VirtualMachine.Config.AddRemoveDevice
```

**Datastore (no propagation):**
```text
Datastore.AllocateSpace
Datastore.Browse
Datastore.FileManagement
```

**Total: ~10 privileges**

#### Cloud Controller Manager (cloud-provider-vsphere)

The Cloud Controller Manager is a **read-only** component that handles node discovery, zone/region topology, and instance metadata. It never creates, modifies, or deletes vSphere objects.

**vCenter Root (no propagation):**
```text
Sessions.ValidateSession
System.Read
InventoryService.Tagging.ObjectAttachable
```

**Datacenter (with propagation):**
```text
System.Read
```

**VM Folder (with propagation):**
```text
VirtualMachine.Config.Query
```

**Cluster/ComputeResource (no propagation):**
```text
Host.Inventory.View
Resource.QueryVMotion
```

**Datastore (no propagation):**
```text
Datastore.Browse
```

**Total: ~10 privileges (read-only)**

*Note: Although the built-in vCenter "Read-only" role includes all privileges listed above, it also grants broader inventory access (e.g., reading all VMs, hosts, and datastores across the entire vCenter). For least-privilege compliance, administrators should create a custom `openshift-cloud-controller` role containing only the privileges listed above. The provided govc and PowerCLI scripts create this custom role. Administrators who accept the broader read access of the built-in role may use it as a convenience alternative.*

#### Diagnostics (vsphere-problem-detector)

**vCenter Root (no propagation):**
```text
Sessions.ValidateSession
System.Read
```

**Datacenter (no propagation):**
```text
System.Read
```

**Datastore (no propagation):**
```text
Datastore.Browse
```

**Total: ~5 privileges (read-only)**

#### Installer

Uses the full privilege set as defined in `installer/pkg/asset/installconfig/vsphere/permissions.go` (~45 privileges).

### Tooling for Administrators

#### govc Script for Role Creation (Multi-vCenter)

```bash
#!/bin/bash
# create-openshift-roles.sh
# Creates vCenter roles for OpenShift components
# Supports multiple vCenters

set -e

# List of vCenter servers to configure
VCENTERS="${VCENTERS:-vcenter1.example.com vcenter2.example.com}"

# Create roles on each vCenter
for VCENTER in $VCENTERS; do
    echo "Creating roles on $VCENTER..."
    export GOVC_URL="$VCENTER"

    # Machine API Role
    govc role.create openshift-machine-api \
    Sessions.ValidateSession \
    InventoryService.Tagging.AttachTag \
    InventoryService.Tagging.CreateTag \
    InventoryService.Tagging.EditTag \
    InventoryService.Tagging.DeleteTag \
    Resource.AssignVMToPool \
    VApp.AssignResourcePool \
    VirtualMachine.Config.AddExistingDisk \
    VirtualMachine.Config.AddNewDisk \
    VirtualMachine.Config.AddRemoveDevice \
    VirtualMachine.Config.AdvancedConfig \
    VirtualMachine.Config.Annotation \
    VirtualMachine.Config.CPUCount \
    VirtualMachine.Config.DiskExtend \
    VirtualMachine.Config.EditDevice \
    VirtualMachine.Config.Memory \
    VirtualMachine.Config.RemoveDisk \
    VirtualMachine.Config.Rename \
    VirtualMachine.Config.ResetGuestInfo \
    VirtualMachine.Config.Resource \
    VirtualMachine.Config.Settings \
    VirtualMachine.Interact.GuestControl \
    VirtualMachine.Interact.PowerOff \
    VirtualMachine.Interact.PowerOn \
    VirtualMachine.Interact.Reset \
    VirtualMachine.Inventory.Create \
    VirtualMachine.Inventory.CreateFromExisting \
    VirtualMachine.Inventory.Delete \
    VirtualMachine.Provisioning.Clone \
    VirtualMachine.Provisioning.DeployTemplate \
    VirtualMachine.State.CreateSnapshot \
    VirtualMachine.State.RemoveSnapshot \
    InventoryService.Tagging.ObjectAttachable \
    Datastore.AllocateSpace \
    Datastore.Browse \
    Datastore.FileManagement \
    Network.Assign

# CSI Driver Role
govc role.create openshift-csi-driver \
    Sessions.ValidateSession \
    Cns.Searchable \
    StorageProfile.View \
    VirtualMachine.Config.AddExistingDisk \
    VirtualMachine.Config.AddRemoveDevice \
    Datastore.AllocateSpace \
    Datastore.Browse \
    Datastore.FileManagement

    # Cloud Controller Manager Role (read-only)
    govc role.create openshift-cloud-controller \
        Sessions.ValidateSession \
        System.Read \
        InventoryService.Tagging.ObjectAttachable \
        VirtualMachine.Config.Query \
        Host.Inventory.View \
        Resource.QueryVMotion \
        Datastore.Browse

    # Diagnostics Role (read-only)
    govc role.create openshift-diagnostics \
        Sessions.ValidateSession \
        System.Read \
        Datastore.Browse

    echo "Roles created on $VCENTER"
done

echo "All roles created successfully on all vCenters"
```

#### Script to Generate ~/.vsphere/credentials

```bash
#!/bin/bash
# generate-vsphere-credentials.sh
# Generates the ~/.vsphere/credentials file for per-component credentials
# Non-destructive: will not overwrite an existing credentials file.

set -euo pipefail
umask 077

CREDS_FILE="${VSPHERE_CREDENTIALS_FILE:-${HOME}/.vsphere/credentials}"
CREDS_DIR="$(dirname "$CREDS_FILE")"

# Guard: refuse to overwrite an existing credentials file
if [[ -f "$CREDS_FILE" ]]; then
    echo "ERROR: Credentials file already exists at $CREDS_FILE" >&2
    echo "Remove or rename it before running this script." >&2
    exit 1
fi

# Create directory only when absent (umask 077 ensures 0700)
if [[ ! -d "$CREDS_DIR" ]]; then
    mkdir -p "$CREDS_DIR"
fi

# Generate credentials file template
cat > "$CREDS_FILE" << 'EOF'
# vSphere Credentials for OpenShift
# Generated by generate-vsphere-credentials.sh
# File permissions: 0600 (readable only by owner)

# Each top-level key is a vCenter server FQDN or IP
vcenter1.example.com:
  # Default credentials (used by installer)
  user: ocp-installer@vsphere.local
  password: REPLACE_WITH_INSTALLER_PASSWORD

  # Per-component credentials
  componentCredentials:
    machineAPI:
      user: ocp-machine-api@vsphere.local
      password: REPLACE_WITH_MACHINE_API_PASSWORD
    csiDriver:
      user: ocp-csi@vsphere.local
      password: REPLACE_WITH_CSI_PASSWORD
    cloudController:
      user: ocp-ccm@vsphere.local
      password: REPLACE_WITH_CCM_PASSWORD
    diagnostics:
      user: ocp-diagnostics@vsphere.local
      password: REPLACE_WITH_DIAGNOSTICS_PASSWORD

# Add additional vCenters as needed:
# vcenter2.example.com:
#   user: ...
#   password: ...
#   componentCredentials:
#     machineAPI:
#       user: ...
#       password: ...
EOF

echo "Credentials template created at $CREDS_FILE"
echo "Please edit the file and replace placeholder passwords"
```

#### PowerCLI Script for Role Creation

```powershell
# Create-OpenShiftRoles.ps1
# Creates vCenter roles for OpenShift components

param(
    [Parameter(Mandatory=$true)]
    [string]$VCenterServer
)

Connect-VIServer -Server $VCenterServer

# Machine API Role
$machineAPIPrivileges = @(
    "Sessions.ValidateSession",
    "InventoryService.Tagging.AttachTag",
    "InventoryService.Tagging.CreateTag",
    "InventoryService.Tagging.EditTag",
    "InventoryService.Tagging.DeleteTag",
    "InventoryService.Tagging.ObjectAttachable",
    "Resource.AssignVMToPool",
    "VApp.AssignResourcePool",
    "VirtualMachine.Config.AddExistingDisk",
    "VirtualMachine.Config.AddNewDisk",
    "VirtualMachine.Config.AddRemoveDevice",
    "VirtualMachine.Config.AdvancedConfig",
    "VirtualMachine.Config.Annotation",
    "VirtualMachine.Config.CPUCount",
    "VirtualMachine.Config.DiskExtend",
    "VirtualMachine.Config.EditDevice",
    "VirtualMachine.Config.Memory",
    "VirtualMachine.Config.RemoveDisk",
    "VirtualMachine.Config.Rename",
    "VirtualMachine.Config.ResetGuestInfo",
    "VirtualMachine.Config.Resource",
    "VirtualMachine.Config.Settings",
    "VirtualMachine.Interact.GuestControl",
    "VirtualMachine.Interact.PowerOff",
    "VirtualMachine.Interact.PowerOn",
    "VirtualMachine.Interact.Reset",
    "VirtualMachine.Inventory.Create",
    "VirtualMachine.Inventory.CreateFromExisting",
    "VirtualMachine.Inventory.Delete",
    "VirtualMachine.Provisioning.Clone",
    "VirtualMachine.Provisioning.DeployTemplate",
    "VirtualMachine.State.CreateSnapshot",
    "VirtualMachine.State.RemoveSnapshot",
    "Datastore.AllocateSpace",
    "Datastore.Browse",
    "Datastore.FileManagement",
    "Network.Assign"
)
New-VIRole -Name "openshift-machine-api" -Privilege (Get-VIPrivilege -Id $machineAPIPrivileges)

# CSI Driver Role
$csiPrivileges = @(
    "Sessions.ValidateSession",
    "Cns.Searchable",
    "StorageProfile.View",
    "VirtualMachine.Config.AddExistingDisk",
    "VirtualMachine.Config.AddRemoveDevice",
    "Datastore.AllocateSpace",
    "Datastore.Browse",
    "Datastore.FileManagement"
)
New-VIRole -Name "openshift-csi-driver" -Privilege (Get-VIPrivilege -Id $csiPrivileges)

# Cloud Controller Manager Role (read-only)
$cloudControllerPrivileges = @(
    "Sessions.ValidateSession",
    "System.Read",
    "InventoryService.Tagging.ObjectAttachable",
    "VirtualMachine.Config.Query",
    "Host.Inventory.View",
    "Resource.QueryVMotion",
    "Datastore.Browse"
)
New-VIRole -Name "openshift-cloud-controller" -Privilege (Get-VIPrivilege -Id $cloudControllerPrivileges)

# Diagnostics Role
$diagPrivileges = @(
    "Sessions.ValidateSession",
    "System.Read",
    "Datastore.Browse"
)
New-VIRole -Name "openshift-diagnostics" -Privilege (Get-VIPrivilege -Id $diagPrivileges)

Write-Host "Roles created successfully on $VCenterServer"
```

### Credential File Security

The `~/.vsphere/credentials` file contains sensitive vCenter credentials. The following security measures are enforced:

1. **File Permissions:** The installer validates that the credentials file has mode `0600` (owner read/write only) and refuses to proceed if permissions are too open.

2. **Directory Permissions:** The `~/.vsphere/` directory should have mode `0700`.

3. **Environment Variable Override:** For CI/CD pipelines, use `VSPHERE_CREDENTIALS_FILE` to point to a credentials file managed by the pipeline's secret management.

4. **Credential Precedence:** Credentials in install-config.yaml take precedence over the credentials file, allowing pipeline automation to override user defaults.

5. **Cleanup:** The installer never modifies the credentials file. Administrators are responsible for credential rotation and cleanup.

### Multi-vCenter Considerations

OpenShift supports spanning multiple vCenters using failure domains. This enhancement ensures per-component credentials work correctly with multi-vCenter topologies:

1. **Per-vCenter Credentials in Each Component Secret:** Each component secret contains separate credentials for each vCenter, allowing:
   - Different identity sources per vCenter
   - Independent password management
   - Flexibility in account naming per vCenter

2. **Credential Validation:** CCO validates each component's credentials on each vCenter independently and reports which vCenter(s) have authentication failures or missing privileges.

3. **Role Consistency:** The same role should be created on each vCenter with the same privileges. The provided scripts handle multi-vCenter role creation.

4. **Secret Key Format:** Credentials are keyed by vCenter FQDN:
   ```yaml
   stringData:
     vcenter1.example.com.username: "user@domain"
     vcenter1.example.com.password: "password1"
     vcenter2.example.com.username: "user@domain"
     vcenter2.example.com.password: "password2"
   ```

5. **Credentials File Format:** The `~/.vsphere/credentials` YAML file similarly organizes credentials by vCenter with per-component entries under each vCenter key.

### Topology Considerations

#### Hypershift / Hosted Control Planes

For Hypershift deployments:
- The management cluster administrator provides credentials for each hosted cluster
- Credentials are stored in the management cluster, scoped per hosted cluster
- Each hosted cluster's components use credentials scoped to that cluster's resources

#### Standalone Clusters

This enhancement fully applies to standalone clusters and is the primary target.

#### Single-node Deployments or MicroShift

For single-node OpenShift (SNO):
- The same per-component credential model applies
- Reduces blast radius even on single-node deployments

For MicroShift:
- This enhancement does not apply; MicroShift does not use CCO

#### OpenShift Kubernetes Engine

Compatible with OKE; does not depend on OCP-specific features beyond CCO.

### Implementation Details/Notes/Constraints

#### Privilege Validation Implementation (vsphere-problem-detector)

Privilege validation is performed by the `vsphere-problem-detector`, not CCO. The following illustrates how the vsphere-problem-detector checks privileges:

```go
// ValidateCredentialPrivileges checks that credentials have required privileges.
// This validation is performed by the vsphere-problem-detector.
func (a *VSphereActuator) ValidateCredentialPrivileges(
    ctx context.Context,
    creds *corev1.Secret,
    required []VSpherePermission,
) error {
    // Connect to vCenter with provided credentials
    client, err := a.connectWithCredentials(ctx, creds)
    if err != nil {
        return fmt.Errorf("failed to connect: %v", err)
    }
    defer client.Logout(ctx)

    authManager := object.NewAuthorizationManager(client.Client)
    sessionMgr := session.NewManager(client.Client)
    userSession, err := sessionMgr.UserSession(ctx)
    if err != nil {
        return fmt.Errorf("failed to retrieve user session: %v", err)
    }
    if userSession == nil {
        return fmt.Errorf("user session is nil; credentials may be invalid or the session expired")
    }

    var missingPrivileges []string

    for _, perm := range required {
        // Get the managed object reference for the scope
        moRef, err := a.resolveScopeToMoRef(ctx, client, perm.Scope)
        if err != nil {
            return fmt.Errorf("failed to resolve scope %v: %v", perm.Scope, err)
        }

        // Fetch user's privileges on this object
        results, err := authManager.FetchUserPrivilegeOnEntities(ctx,
            []types.ManagedObjectReference{moRef},
            userSession.UserName)
        if err != nil {
            return fmt.Errorf("failed to fetch privileges: %v", err)
        }

        userPrivs := sets.NewString()
        for _, result := range results {
            userPrivs.Insert(result.Privileges...)
        }

        // Check each required privilege
        for _, reqPriv := range perm.Privileges {
            if !userPrivs.Has(reqPriv) {
                missingPrivileges = append(missingPrivileges,
                    fmt.Sprintf("%s on %s", reqPriv, perm.Scope.Type))
            }
        }
    }

    if len(missingPrivileges) > 0 {
        return fmt.Errorf("missing privileges: %v", missingPrivileges)
    }

    return nil
}
```

#### Credential Resolution by Mode

CCO resolves credentials based on the configured `credentialsMode`. Root credentials
are only used when Passthrough is explicitly configured (or as the default when no
mode is set). In PerComponent mode, missing or invalid per-component credentials
produce an error rather than silently falling back to the root credential.

```go
func (a *VSphereActuator) GetCredentialsForComponent(
    ctx context.Context,
    component string,
) (*corev1.Secret, error) {
    mode := a.getCredentialsMode(ctx)

    switch mode {
    case VSphereCredentialsModePerComponent:
        // PerComponent mode: per-component credentials are required.
        componentCreds, err := a.getComponentCredentials(ctx, component)
        if err != nil {
            return nil, fmt.Errorf("failed to retrieve credentials for component %s: %v", component, err)
        }
        if componentCreds == nil {
            return nil, fmt.Errorf("no per-component credentials configured for %s in PerComponent mode", component)
        }
        return componentCreds, nil

    case VSphereCredentialsModePassthrough, "":
        // Passthrough mode (explicit or default): use root credentials.
        return a.getRootCredentials(ctx)

    default:
        return nil, fmt.Errorf("unknown credentialsMode %q", mode)
    }
}
```

### Feature Gate

This feature is gated behind the `VSphereMultiAccountCredentials` feature gate. The feature gate controls whether the per-component credential management functionality is active in the cluster.

#### Feature Gate Definition

The `VSphereMultiAccountCredentials` feature gate is defined in `openshift/api` under `features/features.go`:

```go
FeatureGateVSphereMultiAccountCredentials = newFeatureGate("VSphereMultiAccountCredentials").
    reportProblemsToJiraComponent("cloud-credential-operator").
    contactPerson("rvanderp").
    productScope(ocpSpecific).
    enableIn(TechPreviewNoUpgrade, DevPreviewNoUpgrade).
    mustRegister()
```

API fields introduced by this enhancement (`VSpherePlatformSpec.CredentialsMode`, `VSpherePlatformSpec.ComponentCredentials`, and the `VCenterComponentCredentials` structure in install-config) are annotated with `+openshift:enable:FeatureGate=VSphereMultiAccountCredentials`. When the feature gate is not enabled, these fields are stripped from API responses and rejected on admission.

#### Behavior When Feature Gate Is Disabled

When `VSphereMultiAccountCredentials` is not enabled (the default in the `Default` feature set):

- The `credentialsMode` and `componentCredentials` fields on `VSpherePlatformSpec` are not accepted by the API server.
- CCO operates exclusively in Passthrough mode, using the single shared credential (`kube-system/vsphere-creds`) for all components.
- The installer does not read or process `componentCredentials` from install-config.yaml or `~/.vsphere/credentials`.
- Existing clusters continue to function with no behavioral change.

#### Behavior When Feature Gate Is Enabled

When `VSphereMultiAccountCredentials` is enabled (via `TechPreviewNoUpgrade` or `DevPreviewNoUpgrade` feature sets during Tech Preview, or the `Default` feature set after GA promotion):

- The `credentialsMode` and `componentCredentials` fields are accepted on `VSpherePlatformSpec`.
- Administrators can configure per-component credentials as described in this enhancement.
- CCO supports both `Passthrough` and `PerComponent` credential modes.
- The installer reads and processes `componentCredentials` from install-config.yaml and `~/.vsphere/credentials`.

#### Graduation Plan

| Phase | Feature Set | Behavior |
|-------|-------------|----------|
| Tech Preview | `TechPreviewNoUpgrade`, `DevPreviewNoUpgrade` | Feature gate enabled; per-component credentials available for evaluation and testing. No upgrade support. |
| GA | `Default` | Feature gate promoted to `Default` feature set; per-component credentials available on all clusters with full upgrade/downgrade support. |

## Implementation Plan

This section outlines the phased approach to implementing vSphere multi-account credential management across the OpenShift ecosystem. Each phase builds on prior deliverables, and repositories are identified for each phase.

### Phase 0: API Foundation

**Description:** Establish the API types and feature gate that all subsequent phases depend on.

**Key Deliverables:**
- Register `VSphereMultiAccountCredentials` feature gate in `features/features.go`, initially enabled in `TechPreviewNoUpgrade` and `DevPreviewNoUpgrade` feature sets
- Add `VSphereCredentialsMode` enum type with `Passthrough` and `PerComponent` values
- Add `VSphereComponentCredentials` struct with `SecretReference` fields for `MachineAPI`, `CSIDriver`, `CloudController`, and `Diagnostics`
- Add `SecretReference` type with namespace restricted to `openshift-config` via kubebuilder validation
- Add `VSpherePermissionScope` type for privilege scope declarations
- Extend `VSpherePlatformSpec` with `CredentialsMode` and `ComponentCredentials` fields
- Annotate all new fields with `+openshift:enable:FeatureGate=VSphereMultiAccountCredentials`
- Add generated deepcopy methods and OpenAPI schema updates

**Repository:** openshift/api

**Dependencies:** None (foundational phase)

**Estimated Complexity:** Moderate — primarily type definitions and feature gate registration, but requires coordination with API reviewers and generated code updates.

### Phase 1: Installer Support

**Description:** Extend the installer to accept per-component credentials and create the corresponding Kubernetes Secrets during cluster bootstrap.

**Key Deliverables:**
- Extend install-config `VCenter` struct to accept `componentCredentials` per vCenter entry
- Implement reading of per-component credentials from `~/.vsphere/credentials` and `VSPHERE_CREDENTIALS_FILE`
- Implement canonical vCenter key generation with correct normalization for FQDNs, IPv4, and IPv6 addresses (bracket stripping, full expansion, colon-to-dash replacement, non-default port appending)
- Create named Secrets in `openshift-config` namespace during cluster bootstrap (e.g., `vsphere-creds-machine-api`, `vsphere-creds-csi-driver`, `vsphere-creds-cloud-controller`, `vsphere-creds-diagnostics`)
- Set Secret ownership via `metadata.ownerReferences` to the `Infrastructure/cluster` resource
- Populate `Infrastructure/cluster` spec with `credentialsMode: PerComponent` and `componentCredentials` pointing to the created Secrets
- Validate componentCredentials references and vCenter connectivity at install time
- Validate `~/.vsphere/credentials` file permissions (must be 0600)
- Support credential precedence: install-config.yaml > VSPHERE_CREDENTIALS_FILE > ~/.vsphere/credentials

**Repository:** openshift/installer

**Dependencies:** Phase 0 (API types and feature gate must be merged)

**Estimated Complexity:** Primary development effort — credential handoff logic, key normalization, file-format parsing, and multi-vCenter support require significant implementation and testing.

### Phase 2: Cloud Credential Operator (CCO)

**Description:** Implement PerComponent credential mode in CCO to distribute administrator-provisioned credentials to component namespaces.

**Key Deliverables:**
- Implement `PerComponent` credentialsMode handling in the CCO vSphere actuator
- Implement `GetCredentialsForComponent` with explicit mode-based logic: PerComponent mode requires per-component credentials (no fallback to root); Passthrough mode (explicit or default) uses the root credential
- Read per-component `SecretReference` entries from `Infrastructure/cluster` spec
- Enforce that all `SecretReference` entries point to the `openshift-config` namespace; reject references to other namespaces with `CredentialsProvisionFailed`
- Distribute per-component credentials to target namespace Secrets (e.g., `openshift-machine-api/vsphere-cloud-credentials`)
- Validate vCenter connectivity for each credential set on each vCenter in the topology
- Watch for Secret updates in `openshift-config` and propagate changes to component namespaces
- Report per-vCenter authentication failures via `CredentialsProvisionFailed` conditions on `CredentialsRequest` resources
- Support graceful downgrade from PerComponent to Passthrough mode (revert to distributing root credential)

**Repository:** openshift/cloud-credential-operator

**Dependencies:** Phase 0 (API types), Phase 1 (installer creates the source Secrets and sets Infrastructure CR fields)

**Estimated Complexity:** Primary development effort — credential distribution, mode switching, connectivity validation, and Secret watching are core CCO changes.

### Phase 3: vsphere-problem-detector Updates

**Description:** Extend the vsphere-problem-detector to validate per-component credential privilege sets against the requirements defined in this enhancement.

**Key Deliverables:**
- Extend privilege validation to support per-component credential sets in addition to the existing shared-credential checks
- Validate each component's credentials against their specific privilege requirements:
  - Installer: ~45 privileges (full set as defined in `installer/pkg/asset/installconfig/vsphere/permissions.go`)
  - Machine API: ~35 privileges (VM lifecycle, tagging, datastore, network)
  - CSI Driver: ~10-15 privileges (storage operations, CNS)
  - Cloud Controller Manager: ~10 privileges (read-only: node discovery, zone topology)
  - Diagnostics: ~5 privileges (read-only: session, system read, datastore browse)
- Validate privileges per vCenter when multiple vCenters are configured (iterate failure domains)
- Support `InferFromClusterConfig` scope resolution: resolve target objects from the cluster's failure domain configuration
- Validate privilege propagation by checking representative child objects or inspecting permission assignments
- Report per-component validation status via conditions and metrics
- Use `pruneToAvailablePermissions` pattern to handle privilege name differences across vCenter versions

**Repository:** openshift/vsphere-problem-detector

**Dependencies:** Phase 0 (API types for VSpherePermissionScope), Phase 2 (CCO distributes credentials that vsphere-problem-detector validates)

**Estimated Complexity:** Moderate to high — privilege validation logic per component is well-defined, but multi-vCenter iteration and scope resolution add complexity.

### Phase 4: Component Consumers

**Description:** Update each vSphere-consuming component to retrieve credentials via CCO's GetCredentialsForComponent mechanism.

**Key Deliverables:**
- **machine-api-operator:** Update credential retrieval to read from the dedicated per-component Secret (`openshift-machine-api/vsphere-cloud-credentials`) when populated by CCO in PerComponent mode. Fall back gracefully when the feature gate is disabled (continue using the shared credential path).
- **vmware-vsphere-csi-driver:** Update credential retrieval to read from the per-component Secret for CSI operations. Ensure the existing cluster-storage-operator > CVO > CSI operator cloud-credentials contract is preserved.
- **cloud-provider-vsphere (CCM):** Update credential retrieval for the cloud controller manager to use per-component credentials. CCM is read-only so this is primarily a credential source change.
- Each component picks up new credentials on the next vCenter session reconnect without requiring a pod restart.
- Each component operates normally with shared credentials when the feature gate is disabled or credentialsMode is Passthrough.

**Repositories:** openshift/machine-api-operator, openshift/vmware-vsphere-csi-driver, openshift/cloud-provider-vsphere

**Dependencies:** Phase 2 (CCO must be distributing per-component credentials before consumers can use them)

**Estimated Complexity:** Integration work — each consumer change is relatively small (credential source path), but must be validated independently for correct behavior in both PerComponent and Passthrough modes.

### Phase 5: Tooling and Documentation

**Description:** Provide administrator-facing tooling and documentation for creating vCenter roles, accounts, and managing per-component credentials.

**Key Deliverables:**
- Administrator documentation for creating vCenter roles with minimum privileges per component
- govc scripts for multi-vCenter role creation (as outlined in the Tooling for Administrators section)
- PowerCLI scripts for role creation in Windows-centric environments
- Credential generation script template for `~/.vsphere/credentials`
- Day-2 credential rotation procedures (per-component rotation without cluster disruption)
- Upgrade guide: migrating from Passthrough to PerComponent mode on existing clusters
- Downgrade guide: reverting to Passthrough mode and cleanup procedures
- Support runbook for troubleshooting credential-related issues

**Repositories:** openshift/openshift-docs, openshift-splat-team/enhancements

**Dependencies:** Phases 0-4 (documentation must reflect finalized behavior)

**Estimated Complexity:** Low to moderate — documentation and scripting work; scripts are partially drafted in this enhancement.

### Phase 6: Testing and Graduation

**Description:** Implement comprehensive testing and drive the feature through Tech Preview to GA graduation.

**Key Deliverables:**
- **Unit tests** for all modified components:
  - API type validation and serialization (openshift/api)
  - Canonical vCenter key normalization (installer, CCO)
  - GetCredentialsForComponent mode-based resolution (CCO)
  - Privilege validation logic with mock AuthorizationManager (vsphere-problem-detector)
  - Fallback behavior when feature gate is disabled (all consumers)
- **Integration tests:**
  - Credential distribution from openshift-config to component namespaces (CCO)
  - govcsim-based privilege validation (vsphere-problem-detector)
  - Secret update propagation (CCO to component namespaces)
  - Error scenarios: missing privileges, invalid credentials, missing Secrets
- **E2E tests:**
  - Full installation with per-component credentials on multi-vCenter topology
  - Day-2 credential rotation for individual components
  - Migration from Passthrough to PerComponent mode on a running cluster
  - Downgrade from PerComponent to Passthrough mode
  - Verify components use correct credentials via vCenter audit log analysis
  - Tests on both vSphere 7.0 and vSphere 8.0
- **Tech Preview graduation criteria:**
  - Feature gate enabled in `TechPreviewNoUpgrade` and `DevPreviewNoUpgrade` feature sets
  - Basic E2E tests passing in CI
  - User feedback gathered through Tech Preview usage
- **GA graduation criteria:**
  - Feature gate promoted to the `Default` feature set
  - Full upgrade and downgrade testing (Passthrough <-> PerComponent transitions)
  - No P0/P1 bugs outstanding
  - User-facing documentation published in openshift-docs
  - Support runbook in place

**Repositories:** All repositories from prior phases

**Dependencies:** Phases 0-5 (all implementation and documentation complete)

**Estimated Complexity:** High — testing spans all repositories and requires vSphere infrastructure with configurable accounts and multi-vCenter topologies.

### Cross-Cutting Concerns

The following concerns span multiple phases and repositories. Consistent handling across all components is essential.

**Canonical vCenter Key Normalization:**
The canonical key derivation algorithm (strip IPv6 brackets, expand to full 8-group form, replace colons with dashes, lowercase FQDNs, append non-default ports) must produce identical results in the installer, CCO, vsphere-problem-detector, and all component consumers. A shared utility library or function should be used, and the normalization must be covered by unit tests with identical test vectors across all repositories.

**Secret Naming Conventions:**
Secret names must be consistent across the installer (which creates them), CCO (which reads and distributes them), and component consumers (which consume the distributed copies). The naming convention is:
- Source secrets in `openshift-config`: `vsphere-creds-<component>` (e.g., `vsphere-creds-machine-api`, `vsphere-creds-csi-driver`, `vsphere-creds-cloud-controller`, `vsphere-creds-diagnostics`)
- Target secrets in component namespaces: existing names (e.g., `vsphere-cloud-credentials` in `openshift-machine-api`)
- Secret data keys: `<canonical-vcenter-key>.username` and `<canonical-vcenter-key>.password`

**Error Handling and Fallback Behavior:**
All components must follow a consistent error model:
- In PerComponent mode, missing or invalid per-component credentials produce an explicit error — no silent fallback to root credentials.
- In Passthrough mode (explicit or default), the root credential (`kube-system/vsphere-creds`) is used for all components.
- When the feature gate is disabled, components must ignore per-component configuration and operate in Passthrough mode.
- CCO reports failures via `CredentialsProvisionFailed` conditions on `CredentialsRequest` resources. The vsphere-problem-detector reports privilege validation failures via its own conditions and metrics.

**Metrics and Observability:**
- CCO should expose metrics for credential distribution status per component and per vCenter (e.g., `cco_vsphere_credential_distribution_errors_total`).
- The vsphere-problem-detector should expose metrics for privilege validation results per component and per vCenter (e.g., `vsphere_problem_detector_privilege_check_failures_total`).
- All credential-related operations should emit structured log messages that identify the component, vCenter, and operation for troubleshooting.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Administrator provides insufficient privileges | vsphere-problem-detector validates privileges per vCenter and reports specific missing privileges |
| Complex setup burden on administrators | Provide scripts (govc, PowerCLI) and detailed documentation |
| Credential secrets accidentally deleted | Standard Kubernetes secret backup practices; CCO recreates from source |
| vCenter version has different privilege names | Use pruneToAvailablePermissions pattern; validate against actual vCenter |
| Migration from passthrough mode is disruptive | Support gradual migration; components fall back gracefully |
| ~/.vsphere/credentials file exposed | Enforce 0600 permissions; installer refuses to proceed if permissions too open |
| Credentials in install-config.yaml committed to git | Document best practices; warn users about including secrets in version control |
| Multi-vCenter credential mismatch | CCO validates credentials per vCenter before provisioning; clear error messages identify which vCenter failed |
| Inconsistent accounts across vCenters | Provide multi-vCenter scripts that configure all vCenters consistently |

### Drawbacks

1. **Increased Setup Complexity:** Administrators must create multiple accounts/roles
2. **Documentation Overhead:** Must maintain per-component privilege lists
3. **Potential for Misconfiguration:** More credentials means more chances for errors
4. **vCenter Administrative Burden:** More accounts to manage and audit

## Alternatives (Not Implemented)

### Alternative 1: CCO Creates vCenter Accounts (Mint Mode)

**Description:** CCO uses administrative credentials to create per-component accounts automatically.

**Pros:**
- Fully automated
- No manual account creation
- Consistent naming

**Cons:**
- Requires administrative vCenter privileges
- Conflicts with enterprise security practices
- CCO becomes a privileged identity management system

**Why not selected:** Enterprises typically have strict controls on account creation; giving CCO this power is a security concern.

### Alternative 2: Single Account with Multiple Roles

**Description:** Use one account but assign different roles on different vSphere objects.

**Pros:**
- Simpler credential management
- Still provides privilege scoping

**Cons:**
- Single point of compromise
- Cannot distinguish actions in audit logs
- Credential rotation affects all components

**Why not selected:** Does not achieve audit separation goal.

### Alternative 3: External Identity Provider Integration

**Description:** Integrate with external IdP (Vault, CyberArk) for dynamic credentials.

**Pros:**
- Dynamic, short-lived credentials
- Centralized secret management
- Built-in audit

**Cons:**
- Requires external infrastructure
- Adds complexity
- Dependency on third-party system

**Why not selected:** Out of scope; can be future enhancement.

## Open Questions

1. **Validation Frequency:** How often should the vsphere-problem-detector re-validate privileges — periodically, or only on secret changes?

3. **Partial Configuration:** If only some component credentials are provided, should CCO use per-component for those and passthrough for others?

4. **Installer Credentials Lifecycle:** Should installer credentials be automatically disabled post-installation?

5. **Cross-vCenter Account Naming:** Should we recommend the same account name across all vCenters (simpler) or unique names per vCenter (more auditable)?

6. **Credentials File Discovery:** Should the installer search additional locations (e.g., `/etc/vsphere/credentials` for system-wide configuration)?

## Test Plan

### Unit Tests

- VSphereProviderSpec parsing and privilege list handling
- Privilege validation logic with mock AuthorizationManager
- Fallback to passthrough mode
- Error messaging for missing privileges

### Integration Tests

- Credential validation using govcsim
- Per-component secret distribution
- Mixed mode (some per-component, some passthrough)
- Privilege validation failure scenarios

### E2E Tests

- Full installation with per-component credentials
- Verify components use correct credentials (audit log verification)
- Credential rotation for individual components
- Migration from passthrough to per-component mode

## Graduation Criteria

### Dev Preview -> Tech Preview

- `VSphereMultiAccountCredentials` feature gate enabled in `TechPreviewNoUpgrade` and `DevPreviewNoUpgrade` feature sets
- Per-component credential configuration supported behind the feature gate
- API fields gated with `+openshift:enable:FeatureGate=VSphereMultiAccountCredentials`
- Privilege validation implemented in vsphere-problem-detector
- Documentation for creating roles/accounts
- Scripts for govc and PowerCLI
- Sufficient test coverage (unit and integration)
- Gather feedback from users rather than just developers

### Tech Preview -> GA

- Promote `VSphereMultiAccountCredentials` feature gate to the `Default` feature set
- E2E tests in CI passing reliably
- Tested on vSphere 7.0 and 8.0
- Upgrade and downgrade testing (Passthrough ↔ PerComponent transitions)
- User-facing documentation created in [openshift-docs](https://github.com/openshift/openshift-docs/)
- Migration guide from passthrough mode
- Support runbook
- Sufficient time for Tech Preview feedback
- No P0/P1 bugs outstanding

## Upgrade / Downgrade Strategy

### Upgrade (Passthrough → PerComponent)

1. Administrator creates per-component accounts
2. Administrator creates per-component secrets
3. Administrator updates Infrastructure CR to PerComponent mode
4. CCO validates and distributes new credentials
5. Components pick up new credentials

### Downgrade (PerComponent → Passthrough)

1. Administrator updates the `Infrastructure/cluster` CR, setting `credentialsMode` to `Passthrough`.
2. CCO detects the mode change during reconciliation, stops reading per-component `SecretReference` entries, and reverts to distributing the root credential (`kube-system/vsphere-creds`) to all component namespaces.
3. Components pick up the root credential on their next reconciliation cycle.

**Per-component Secret cleanup:**

- The per-component Secrets in `openshift-config` (e.g., `vsphere-creds-machine-api`) are **not** automatically deleted during downgrade. CCO leaves them in place so that the administrator can re-enable PerComponent mode without re-creating Secrets.
- CCO removes the `componentCredentials` field from the `Infrastructure` spec only when the administrator explicitly clears it. Until then, the references remain as documentation of the previous configuration.
- The administrator is responsible for deleting unused per-component Secrets when they are no longer needed. The support procedures section provides commands for identifying and removing these Secrets.
- If the administrator re-enables PerComponent mode before deleting the Secrets, CCO verifies connectivity for the existing credentials and resumes per-component distribution without data loss.

**Rollback safety:**

- During the transition window (after mode change, before all components reconcile), some components may still hold per-component credentials while others have already picked up the root credential. Both credential sets remain valid during this period because the root credential is a superset of all per-component privileges.
- If the downgrade is reverted (switching back to PerComponent), CCO resumes using the existing per-component Secrets, provided they have not been deleted.

## Version Skew Strategy

- New CCO version is backward compatible with passthrough mode
- API extensions are additive (new fields, not breaking changes)
- Components that don't understand per-component mode use existing secret paths

## Operational Aspects of API Extensions

### New API Fields

- `VSpherePlatformSpec.CredentialsMode`: Enum, no webhook needed
- `VSpherePlatformSpec.ComponentCredentials`: Reference to secrets

### Impact on SLIs

- Minimal additional vCenter API calls for credential distribution and connectivity verification
- The vsphere-problem-detector performs privilege validation as part of its existing checks

### Failure Modes

| Failure | Detection | Resolution |
|---------|-----------|------------|
| Missing privileges | CCO condition `CredentialsProvisionFailed` | Add missing privileges to vCenter role |
| Invalid credentials | CCO logs authentication failure | Verify account and password |
| Secret not found | CCO condition reports missing secret | Create the referenced secret |

## Support Procedures

### Detecting Issues

```bash
# Check CredentialsRequest status
oc get credentialsrequest -n openshift-cloud-credential-operator -o yaml

# Check CCO logs
oc logs -n openshift-cloud-credential-operator deployment/cloud-credential-operator

# Verify component secrets exist (metadata only — never use -o yaml which dumps values)
oc get secret -n openshift-machine-api vsphere-cloud-credentials
# To inspect key names without exposing values:
oc get secret -n openshift-machine-api vsphere-cloud-credentials \
    -o go-template='{{range $k, $v := .data}}{{$k}}{{"\n"}}{{end}}'
```

### Reverting to Passthrough Mode

```bash
# Update Infrastructure CR
oc patch infrastructure cluster --type=merge -p '
{
  "spec": {
    "platformSpec": {
      "vsphere": {
        "credentialsMode": "Passthrough"
      }
    }
  }
}'
```

## Infrastructure Needed

- vSphere test environments with configurable accounts
- CI integration for privilege validation testing
- Documentation site updates for per-component setup guide
