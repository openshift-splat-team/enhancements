---
title: bmc-ca-certificate-support
authors:
  - "@Hellcatlk"
  - "@zhouhao3"
  - "@fenggw-fnst"
reviewers:
  - "@dtantsur"
  - "@zaneb"
approvers:
  - "@dtantsur"
api-approvers:
  - "@dtantsur"
tracking-link:
  - "https://issues.redhat.com/browse/RFE-3505"
creation-date: 2023-04-13
last-updated: 2023-09-22
---

# BMC CA Certificate Support

## Summary

This enhancement allows users to provide local CA certificates for IPI
installation or subsequent node management, ensuring secure communication
with baremetal BMCs that use certificates issued by the local CA or use
self-signed certificates.

## Motivation

Currently, when deploying OCP clusters on baremetal using IPI or managing nodes
on an existing cluster, users are unable to specify CA certificates. They can
either use the system's default CA bundle for validation (which means they
need to install certificates signed by a trusted CA for their BMCs) or disable
certificate validation by setting disableCertificateVerification to false in
BareMetalHost. This poses a security risk, particularly in production
environments where such risks are unacceptable.

### User Stories

- As a cluster creator, I want to provide local CA certificates for IPI
  installation, so that I can ensure secure communication with baremetal
  BMCs using certificates issued by the local CA.

- As a cluster creator, I want to utilize self-signed certificates for
  baremetal BMCs during the IPI installation process, so that I can avoid
  relying on external CAs for secure communication.

- As a cluster administrator, I want to add new nodes to an existing cluster
  securely using local-CA-signed certificates or self-signed certificates, so
  that I can maintain a high level of security within the cluster.

- As a cluster administrator, I want to update the CA certificates to address
  various situations, such as renewing the validity of self-signed certificates
  or trusting additional certificates issued by other local CAs, so that I can
  ensure continued secure communication with BMCs.

### Goals

- Administrator provide CA certificates for IPI installation or node management after
  IPI, and these certificates successfully validate the SSL connections with
  baremetal BMCs.

- Administrator update the CA certificates for their existing clusters, and the updated
  certificates successfully validate the SSL connections with baremetal BMCs.

### Non-Goals

- Allow administrator to modify the CA certificate during the IPI installation process.

- Provide an automatic CA certificate issuance feature for IPI installation or
  cluster node management.

- Support certificate validation for devices or components other than
  baremetal BMCs.

## Proposal

- Modify [installer][OpenShift Installer] for control plane installation:
  - Add a new optional field `platform.baremetal.bmcCACert` in
    `install-config.yaml` to allow users to enter the contents of the CA
    certificates.
  - Perform a pre-validation of the SSL connection with the BMCs using the
    provided CA certificates before the installation begins. If the validation
    fails, a warning message will be outputed to indicate which baremetal
    failed. This helps to identify issues early on, especially for the
    time-consuming IPI installation.
  - Modify `startironic.sh.template`:
    - Create the CA certificate file in of the bootstrap VM
      according to `platform.baremetal.bmcCACert`.
    - Mount the CA certificate file when starting the Ironic container.
    - Create the contents of CA certificate file as a ConfigMap.

- Modify [ironic-image][Metal3 Ironic Container] to detect whether the CA certificate file
  is successfully mounted into the container. If this file exists, set the path of this
  file to the value of `default_verify_ca_path` in `ironic.conf`.

- Modify [OpenStack Ironic][OpenStack Ironic] to accept CA path in its
  configuration (Implementation details will be discussed in the ironic community).

- Modify [cluster-baremetal-operator][OpenShift Cluster Baremetal Operator]
  to ensure mounting the `bmc-verify-ca` ConfigMap into Ironic container
  for worker node deployment, and unmounting it when `bmc-verify-ca`
  ConfigMap is deleted.

### Workflow Description

**cluster creator** is a human user responsible for deploying a cluster.

**cluster administrator** is a human user responsible for managing a cluster.

- Deploy a cluster via IPI installation:
  1. The cluster creator enters the contents of the CA certificate under the
     `platform.baremetal.bmcCACert` field in `install-config.yaml`.
  2. The cluster creator ensures that `disableCertificateVerification`
     is not set to true in `install-config.yaml`.
  3. The cluster creator executes the installer command to install the cluster.

- Update the CA certificate for an existing cluster:
  1. The cluster administrator modifies the contents of the `bmc-verify-ca`
     ConfigMap in `openshift-machine-api` namespace and applies the changes.

- Enable secure SSL communication with BMCs for an existing cluster without
  initial CA certificate configuration:
  1. The cluster administrator creates a ConfigMap named `bmc-verify-ca` in
     `openshift-machine-api` namespace, containing the desired CA certificates.
  2. The cluster administrator confirms that the `bmc-verify-ca` ConfigMap has
     been mounted to `/certs/ca/bmc` in Ironic container.
  3. The cluster administrator ensures that `disableCertificateVerification`
     is not set to true in BareMetalHost objects.

### API Extensions

Add an optional string field `bmcCACert` under `platform.baremetal` in
`install-config.yaml`. Administrators can add the CA certificates used in
cluster deployment to this field.

```diff
 # install-config.yaml
 platform:
   baremetal:
     apiVIP: xxx.xxx.xxx.xxx
     ingressVIP: xxx.xxx.xxx.xxx
     provisioningNetworkCIDR: xxx.xxx.xxx.xxx/xx
+    bmcCACert:
+      -----BEGIN CERTIFICATE-----
+      ......
+      ......
+      ......
+      -----END CERTIFICATE-----
+      -----BEGIN CERTIFICATE-----
+      ......
+      ......
+      ......
+      -----END CERTIFICATE-----
+      -----BEGIN CERTIFICATE-----
+      ......
+      ......
+      ......
+      -----END CERTIFICATE-----
+      ......
     ......
     hosts:
         - name: master-0
           bmc:
             disableCertificateVerification: false
         ......
     ......
```

### Risks and Mitigations

Users may incorrectly configure the CA certificates, causing the IPI
installation to timeout. It may take some time for users to realize that the
failure is due to incorrect certificate configuration.

To mitigate this, we can perform a pre-validation of the SSL connection with
the BMCs using the provided CA certificate before the installation begins.

### Drawbacks

- Introducing CA certificate support increases complexity to the installation
  and management process.

- Creating or deleting the CA certificate ConfigMap in an existing cluster
  causes the Ironic container to restart, leading to temporary unavailability.

- CA certificate pre-validation performed in installer is only for cluster
  installation. There is no automatic pre-validation process for CA certificate
  updates in an existing cluster.

## Design Details

### Test Plan
- Unit Test
  - Add UT for CBO.

- E2E test
  - Configure a Redfish job to interact with Ironic.
  - Set up sushy-tools to use a self-signed certificate, ensuring it is configured to work
    alongside the Redfish job.
  - Provide the bmcCACert to ironic and ensure that certificate validation is enable.
  - Execute the end-to-end test to verify if the cluster deployment is successful.
    Successful deployment would indirectly suggest that the certificate configuration is
    correctly recognized and utilized by Ironic.

### Graduation Criteria

This work will be GA immediately since there is no phasing possible or planned.

#### Dev Preview -> Tech Preview

N/A

#### Tech Preview -> GA

N/A

#### Removing a deprecated feature

N/A

### Upgrade / Downgrade Strategy

This enhancement should not affect the Upgrade/Downgrade Strategy.

### Version Skew Strategy

This enhancement requires the support of both `cluster-baremetal-operator`
and `ironic-image`. The version of ironic-image is controlled by
`cluster-baremetal-operator`, thus there will be no version skew.

### Operational Aspects of API Extensions

N/A

#### Failure Modes

1. Before the IPI installation begins, the `installer` will perform a
   pre-validation using the provided CA certificate. If the validation fails,
   an error will be reported immediately, indicating which baremetal BMC failed
   the validation, and the installation will not proceed.

2. If the pre-validation passes, but a certificate validation error occurs
   during the installation process (e.g., due to the changes to the baremetal
   BMC certificate or a man-in-the-middle attack), the user will not be able to
   see the error message directly until timeout. However, if the user notices
   that the installation duration is abnormal, they can check the `ironic`
   container for error messages similar to the following:

   ```log
   ERROR ironic.conductor.utils requests.exceptions.SSLError: HTTPSConnectionPool(host='xxx.xxx.xxx.xxx', port=443): Max retries exceeded with url: /rest/v1/Oem/eLCM/ProfileManagement/BiosConfig (Caused by SSLError(SSLError(524297, '[X509] PEM lib (_ssl.c:4293)')))
   ```

#### Support Procedures

TBD

## Implementation History

TBD

## Alternatives

An alternative approach would be to add these CA certificates through the `additionalTrustBundle`.
However, this has been discussed in a similar [enhancement][OpenStack enhancement]
and was deemed not to be the optimal way.
For our enhancement, there are two primary reasons to consider:
1. It cannot be applied to the BMC certificate verification during control plane creation.
2. There is also the `additionalTrustBundlePolicy` that can be set to either Always or Proxyonly,
  in the latter case using it for the BMCs would not make sense (and this would leave us without
  another way to set the trust bundle for the BMCs).

[OpenStack Ironic]: https://opendev.org/openstack/ironic
[Metal3 Ironic Container]: https://github.com/metal3-io/ironic-image
[OpenShift Installer]: https://github.com/openshift/installer
[OpenShift Cluster Baremetal Operator]: https://github.com/openshift/cluster-baremetal-operator
[OpenStack enhancement]: https://github.com/openshift/enhancements/blob/master/enhancements/security/openstack-self-signed-certificates.md#how-should-we-pass-the-certificate-to-the-installer
