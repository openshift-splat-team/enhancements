---
title: support-security-group-default-router
authors:
  - TBD
reviewers: # Include a comment about what domain expertise a reviewer is expected to bring and what area of the enhancement you expect them to focus on. For example: - "@networkguru, for networking aspects, please look at IP bootstrapping aspect"
  - @rvanderp3
approvers: # A single approver is preferred, the role of the approver is to raise important questions, help ensure the enhancement receives reviews from all applicable areas/SMEs, and determine when consensus is achieved such that the EP can move forward to implementation.  Having multiple approvers makes it difficult to determine who is responsible for the actual approval.
  - @patrick   # Installer changes
  - @joelspeed # API and CCM changes
  - @miciah    # CIO changes
  - # ROSA Classic
  - # ROSA HCP

api-approvers: # In case of new or modified APIs or API extensions (CRDs, aggregated apiservers, webhooks, finalizers). If there is no API change, use "None"
  - TBD
creation-date: yyyy-mm-dd
last-updated: yyyy-mm-dd
tracking-link: # link to the tracking ticket (for example: Jira Feature or Epic ticket) that corresponds to this enhancement
  - TBD
see-also:
  - "/enhancements/this-other-neat-thing.md"
replaces:
  - "/enhancements/that-less-than-great-idea.md"
superseded-by:
  - "/enhancements/our-past-effort.md"
---

# Supporting Security Groups for NLBs on AWS through Ingress

> WIP/TODO

## Summary

> WIP/TODO

<!-- Support for Security Groups on Network Load Balancers (NLB) through default Ingress when deploying an OpenShift cluster on AWS. -->

This enhancement proposes adding support for automatically creating and managing AWS Security Groups for Network Load Balancers (NLBs) used by the default OpenShift Ingress Controller when deployed on AWS. This feature will be opt-in via a configuration setting in the install-config.yaml, allowing administrators to enhance the security of their ingress traffic by leveraging NLB Security Groups, similar to how Security Groups are managed for Classic Load Balancers (CLBs) today. The implementation will primarily involve changes within the OpenShift Cloud Controller Manager (CCM) and the Cluster Ingress Operator (CIO).

## Motivation

Customers deploying OpenShift on AWS using Network Load Balancers (NLBs) for the default router have expressed the need for a similar security configuration as provided by Classic Load Balancers (CLBs), where a security group is directly associated with the load balancer. This allows for more granular control over inbound and outbound traffic at the load balancer level, aligning with AWS security best practices and addressing security findings that flag the lack of security groups on NLBs provisioned by the default CCM.

The default router in OpenShift, a IngressController object managed by Cluster Ingress Controller Operator (CIO), can be created with a Service type Load Balancer Network Load Balancer (NLB) instead of default Classic Load Balancer (CLB) during installation by enabling it in the install-config.yaml. Currently, the Cloud Controller Manager (CCM), which satisfies Service resources, provisions an AWS Load Balancer of type NLB without a Security Group (SG) directly attached to it. Instead, security rules are managed on the worker nodes' security groups.

AWS announced support for Security Groups when deploying a NLB in August 2023, and the CCM for AWS (within kubernetes/cloud-provider-aws ) does not currently implement the feature of automatically creating and managing a security groups for service type LoadBalancer NLB. While the AWS Load Balancer Controller (ALBC/LBC) already supports deploying security groups for NLBs, this enhancement focuses on adding minimal, opt-in support to the existing CCM to address immediate customer needs without a full migration to the LBC. This approach aims to provide the necessary functionality without requiring significant changes in other OpenShift components like the Ingress Controller, installer, ROSA, etc.

Using a Network Load Balancer is a recommended network-based Load Balancer by AWS , and attaching a Security Group to an NLB is a security best practice . NLBs also do not support attaching security groups after they are created.

[1] TBD annoucement URL
[aws-lbc]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/

### User Stories

- As an OpenShift administrator, I want to deploy a cluster on AWS (self-managed, ROSA Classic, and ROSA HCP) using a Network Load Balancer with Security Groups in the default router service, so that I can comply with AWS best practices and "security findings"[1].

- As a Developer I want to create an IngressController on OpenShift on AWS using NLB and Security Group

- As a Developer I want to deploy a service type Load Balancer with NLB and security groups (without additional controllers)

- As an OpenShift Engineer of CIO I want the CCM to manage the Security Group when creating service type LB NLB, so it decreases: a) the amount of provider-specific changes on CIO; b) decrease the amount of maitained code/projects by the team; c) enhance configurations to the Ingress; d) decrease the amount of images in the core payload;

- As an OpenShift Engineer, I want to provide an opt-in mechanism for using NLBs with managed Security Groups for the default router, paving the way for potentially making NLB the default LB type in the future.

[1] TODO: "Security Findings" need to be expanded to collect exact examples. This comes from the customer's comment: https://issues.redhat.com/browse/RFE-5440?focusedId=25761057&page=com.atlassian.jira.plugin.system.issuetabpanels:comment-tabpanel#comment-25761057s


### Goals

**Opt-in NLB provisioning with Security Groups for Default Ingress with FULL SG control by CCM**.

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) by setting a configuration in the `install-config.yaml`. The Cluster Ingress Operator (CIO) will create the service manifest with a new annotation to signal the CCM to create and fully manage the Security Group lifecycle for the NLB (similar to CLB, but as opt-in through annotation).

e2e PoC: https://github.com/openshift/installer/pull/9681

Proposed Phases:

**Phase 1: Create support on Self-Managed**

- OpenShift Install configures the IngressController `default` to use NLB with Security Groups.
- OpenShift Cluster Ingress Controller creates a service type Load Balancer with annotations to use NLB with managed Security Group
- AWS Cloud Controller Manager (CCM) creates the AWS Network Load Balancer with Security Group.

**Phase 2: Create support on ROSA Classic**

- TBD: Investigate how ROSA Classic (Hive?) reads and reacts to the new option in `install-config`: `platform.aws.ingressController.managedSecurityGroup`. Ensure the CIO manifests are generated correctly.

**Phase 3: Create support on ROSA HCP**

- TBD: Investigate how Hypershift sets the Service Annotation for Security Group management when launching CIO for Hosted Control Planes.
- TBD: Explore the HCP flow to validate if the self-managed proposed covers it or if specific adjustments are needed.

### Non-Goals

- Migrate to use ALBC as the provisioner on CIO for the default router service (See more in Alternatives).
- Use NLB as the default service type LoadBalancer by the default router.
- Synchronize all NLB features from ALBC to CCM.
- Change the existing CCM flow when deploying NLB without the new opt-in annotation.
- Change the default OpenShift install flow when deploying the default router using IPI (the opt-in flag must be explicitly enabled).

## Proposal

> WIP/TBR

**Opt-in NLB provisioning with Security Groups for Default Ingress with FULL SG control by CCM**.

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) by setting a configuration in the `install-config.yaml`. The Cluster Ingress Operator (CIO) will create the service manifest with a new annotation to signal the CCM to create and fully manage the Security Group lifecycle for the NLB (similar to CLB, but as opt-in through annotation).

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB.
- Focus on customer scalability when enabling SG on NLB.
- Moderate changes to CCM.

Risk:
- CCM/upstream:
  - SG management increases controller complexity and scenarios to validate.
  - API changes (new annotation).
  - More changes upstream increase the consensus/approvals required, especially for new features in service LB on CCM (prefer ALBC in the long term).
  - More changes in CCM to create and manage the SG lifecycle.
- CCM/downstream:
  - More complex to maintain downstream if not fully upstreamed.

Day-2:
- Self-managed: Patch CIO.
- Managed Services: Patch CIO.
- Updates trigger service recreation of NLB (?).


Supperficial mapping components complexity / T-Shirt Sizing:

| Component  | T-Shirt Size | Complexity | Note                                                                 |
|------------|--------------|------------|----------------------------------------------------------------------|
| CCM        | M            | M          | API introduces annotation to "create SG on NLB" (opt-in). Creates and manages SG lifecycle (creation, rules, deletion). |
| CIO        | S            | S          | API adds flag to enable SG management on NLB.                        |
| Installer  | S            | S          | Reads install-config and sets the flag in CIO manifests.             |
| ROSA CL    | S?           | S?         | Reads/reacts to new option in install-config.                        |
| ROSA HCP   | S?           | S?         | Sets the flag in CIO manifests based on install-config.              |
| Day-2      | S            | S          | Patch CIO to recreate NLB with/without SG.                           |


Open questions:
- Considering this pattern is preferable for security reasons, do we need a plan to enable this flow by default when using NLB in the future?
- Can we start deploying self-managed by default IPI with NLB with managed Security Groups? Is there any blocking considering NLB is already a default flow? Classic is discontinued (not recommended to be used by AWS) from a long time.

e2e PoC: https://github.com/openshift/installer/pull/9681

Proposed Phases:

**Phase 1: Create support on Self-Managed**

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the `install-config.yaml`.

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB
- Focus on customer scalability when enabling SG on NLB.
- Minimal changes to CCM for initial support.

Goals:
- Installer manifest stage: install-config update to include a flag (`platform.aws.ingressController.managedSecurityGroup`) to enable Security Group management for NLB. The installer updates the CIO manifests based on this flag.
- Cluster-Ingress-Operator:
  - Introduce a new field (`managedSecurityGroup`) in the NLB provider parameters within the IngressController API.
  - Controller creates the Service for the default router with the annotation `service.beta.kubernetes.io/aws-load-balancer-type: nlb` and the new annotation `service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true"` when the `managedSecurityGroup` flag is enabled.
- Cloud Controller Manager (CCM):
  - Enable support for the new annotation `service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true"` when creating a service type Load Balancer with `aws-load-balancer-type: nlb`.
  - When the annotation is present, the CCM will:
    - Create a new Security Group instance for the NLB.
    - Create Ingress and Egress rules in the Security Group based on the NLB Listeners and Target Groups' ports. Initially, egress rules should be restricted to the necessary ports for backend communication (traffic and health check ports).
    - Pass the ID of the newly created Security Group during the NLB creation via the AWS API.
    - Delete the Security Group when the corresponding service is deleted.
  - Enhance existing tests for the Load Balancer component in the CCM to include scenarios with the new annotation.


**Phase 2: Create support on ROSA Classic**

Goals:
- TBD: Investigate how ROSA Classic (Hive?) reads and reacts to the new option in `install-config`: `platform.aws.ingressController.managedSecurityGroup`. Ensure the CIO manifests are generated correctly.

**Phase 3: Create support on ROSA HCP**

Goals:
- TBD: Investigate how Hypershift sets the Service Annotation for Security Group management when launching CIO for Hosted Control Planes.
- TBD: Explore the HCP flow to validate if the self-managed proposed covers it or if specific adjustments are needed.


### Workflow Description

> WIP

#### OpenShift Self-managed

- User:
  - Creates `install-config.yaml` enabling the use of Security Group, example `platform.aws.ingressController.securityGroupEnabled`, **when** `lbType=NLB` (already exists).
```yaml
# install-config.yaml
platform:
  aws:
    region: us-east-1
    lbType: NLB           <-- what about to deprecate this field in favor of platform.aws.ingressController.loadBalancerType?
    ingressController:            <-- proposing to aggregate CIO configurations
      securityGroupEnabled: True  <-- new field
[...]
```
- Installer:
  - The installer will generate the CIO manifests, setting the `managedSecurityGroup` field in the `networkLoadBalancer` section of the IngressController `spec`:
```yaml
# install-config.yaml
$ yq ea . $INSTALL_DIR/manifests/cluster-ingress-default-ingresscontroller.yaml
apiVersion: operator.openshift.io/v1
kind: IngressController
metadata:
  creationTimestamp: null
  name: default
  namespace: openshift-ingress-operator
spec:
[...]
  endpointPublishingStrategy:
    loadBalancer:
      providerParameters:
        aws:
          networkLoadBalancer:
            managedSecurityGroup: true   <-- new field
          type: NLB
        type: AWS
      scope: External
    type: LoadBalancerService
[...]
```
- Cluster Ingress Operator (CIO):
  - CIO will create the Service instance for the default router. If the `spec.endpointPublishingStrategy.loadBalancer.providerParameters.aws.networkLoadBalancer.managedSecurityGroup` is `true`, CIO will include the following annotation in the Service manifest:
```yaml
# Manifest for Service XYZ is created with annotations:
apiVersion: v1
kind: Service
metadata:
  name: echoserver
  namespace: mrbraga
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
    service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
    service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true" <-- new annotation
[...]
```
- Cloud Controller Manager (CCM):
  - CCM validates the annotation to manage SG on NLB, creates the SG and rules, and pass the SG ID to LB creation.
  - The annotation `service.beta.kubernetes.io/aws-load-balancer-managed-security-group` (new) msut be set to `true`, then creates the SG with required rules for ingress (based on listeners) and egress (based on service and health check ports).
  - CCM LB controller manages the SG lifecycle (controllers may exists in CLB).

  - The CCM's service controller will watch for Service creations and updates.
  - When it encounters a Service with the annotation `service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true"` and `service.beta.kubernetes.io/aws-load-balancer-type: nlb`, the CCM will:
    - Create a new AWS Security Group for the NLB. The name should follow a convention like `k8s-elb-a<generated-name-from-service-uid>`.
    - Configure Ingress rules in the Security Group to allow traffic on the ports defined in the Service's `spec.ports`. The source for these rules will be determined by the `service.beta.kubernetes.io/load-balancer-source-ranges` annotation on the Service (if present, otherwise default to allowing from all IPs).
    - Configure Egress rules in the Security Group to allow traffic to the backend pods on the targetPort specified in the Service's `spec.ports` and the health check port. Initially, this should be restricted to the cluster's VPC CIDR or the specific CIDRs of the worker nodes.
    - When creating the NLB using the AWS ELBv2 API, the CCM will include the ID of the newly created Security Group in the `SecurityGroups` parameter of the `CreateLoadBalancerInput.`
  - When the Service is deleted, the CCM will also delete the associated Security Group, ensuring proper cleanup.

#### OpenShift Managed (TBD)

- ROSA Classic:
    - TBD: Ensure the `install-config.yaml` option correctly writes the CIO manifests enabling NLB with managed Security Groups. This might involve changes in the Hive component responsible for cluster provisioning.

- ROSA HCP:
    - TBD: Investigate how Hypershift sets the Service Annotation of Security Group management when launching CIO. The `install-config.yaml` option should influence the Hosted Control Plane's CIO configuration. Need to validate if ROSA HCP follows the pattern of Hypershift or require additional configuration.


### API Extensions

> WIP/TODO

#### AWS Cloud Controller Manager (CCM)

- Introduction of a new annotation constant: `ServiceAnnotationLoadBalancerManagedSecurityGroup = "service.beta.kubernetes.io/aws-load-balancer-managed-security-group"` in `pkg/providers/v1/aws.go`.

```go
// ServiceAnnotationLoadBalancerManagedSecurityGroup is the annotation used
// on the service to specify the instruct CCM to manage the security group when creating a Network Load Balancer. When enabled,
// the CCM creates the security group and it's rules. This option can not be used with annotations
// "service.beta.kubernetes.io/aws-load-balancer-security-groups" and "service.beta.kubernetes.io/aws-load-balancer-extra-security-groups".
const ServiceAnnotationLoadBalancerManagedSecurityGroup = "service.beta.kubernetes.io/aws-load-balancer-managed-security-group"
```

- Logic in the service controller within the CCM (`pkg/providers/v1/aws.go` and `pkg/providers/v1/aws_loadbalancer.go` ) to recognize and handle the new annotation when the service type is `NLB` (`ServiceAnnotationLoadBalancerType = "service.beta.kubernetes.io/aws-load-balancer-type"` ).
- Functionality within the CCM to create and manage the lifecycle of AWS Security Groups for NLBs, including creating ingress and egress rules based on the service specification. This would likely involve using the AWS SDK for Go to interact with the EC2 API for creating and managing security groups.

#### Cluster Ingress Operator (CIO)

- FeatureGate TP
- Receive an flag to enable Security Groups on Network Load Balancer structure

- Introduce a new boolean field `managedSecurityGroup` in the `NetworkLoadBalancer` provider parameters within the IngressController API (`operator.openshift.io/v1`).

```go
// AWSNetworkLoadBalancerParameters holds configuration parameters for an
// AWS Network load balancer. For Example: Setting AWS EIPs https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/elastic-ip-addresses-eip.html
type AWSNetworkLoadBalancerParameters struct {
	// managedSecurityGroup specifies whether the service load balancer should create
	// and manage security groups for the Network Load Balancer.
	//
	// +optional
	// +openshift:enable:FeatureGate=IngressNLBSecurityGroup
	ManagedSecurityGroup bool `json:"managedSecurityGroup,omitempty"`
}
```

- The CIO controller will set the `service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true"` annotation on the default router Service if the `managedSecurityGroup` field in the IngressController spec is set to true.

#### Installer

- Introduce a new boolean field `managedSecurityGroup` under `platform.aws.ingressController` in the `install-config.yaml`. This field will only be considered when `platform.aws.lbType` is `NLB`.

```go
// Platform stores all the global configuration that all machinesets
// use.
type Platform struct {
	// IngressController is an optional extra configuration for the Ingress Controllers.
	// +optional
	IngressController *IngressController `json:"ingressController,omitempty"`
}

// IngressController specifies the additional ingress controller configuration.
type IngressController struct {
	// SecurityGroupEnabled is an optional field to enable security groups
	// when using load balancer type Network Load Balancer (NLB).
	// When this field is enabled with LBType NLB, all ingresscontrollers (including the
	// default ingresscontroller) will be created using security group in the NLBs
	// by default.
	//
	// If this field is not set explicitly, it defaults to no security groups in the NLB.
	// This default is subject to change over time.
	//
	// +optional
	SecurityGroupEnabled bool `json:"securityGroupEnabled,omitempty"`
}
```

- TBD: deprecate the field `platform.aws.lbType` in favor of `platform.aws.ingressController.loadBalancerType`

#### ROSA Classic

- TBD: API changes in Hive to read and process the new install-config option.

#### Hypershift/ROSA HCP

- TBD: API changes in Hypershift to configure CIO based on the install-config option.

### Topology Considerations

#### Hypershift / Hosted Control Planes

- TODO/TBD: The flow needs to be explored to determine how the install-config option propagates to the Hosted Control Plane's CIO configuration. This might involve setting the annotation on the Service created in the guest cluster.

#### Standalone Clusters

<!-- Is the change relevant for standalone clusters? -->

> TODO/TBD: All changes is proposed initially and exclusively for Standalone clusters.


#### Single-node Deployments or MicroShift

> TODO/TBD


### Implementation Details/Notes/Constraints

> WIP/TBReview

- The implementation in CCM should handle the case where the `service.beta.kubernetes.io/aws-load-balancer-managed-security-group` annotation is set to `true` but the service type is not `NLB` (`aws-load-balancer-type: nlb`). In this scenario, the CCM should likely log a warning or an error and not attempt to create a security group.
- The initial implementation will focus on creating a single Security Group per NLB.
- The Security Group naming convention should be consistent and informative, including the cluster ID, namespace, and service name to aid in identification and management in the AWS console.
- Egress rule management in CCM needs careful consideration to avoid overly permissive rules. The initial implementation should restrict egress to the necessary ports and protocols for communication with the backend pods (traffic ports and health check ports) within the cluster's VPC.
- Proper IAM permissions for the CCM's service account will be required to allow it to create, describe, and delete Security Groups in AWS. This needs to be documented as a prerequisite.

### Risks and Mitigations

> WIP/TBR

- **Increased complexity in CCM**: Adding security group management to CCM increases its complexity. Mitigation: Focus on a minimal and well-tested implementation, drawing inspiration from the existing CLB security group management logic in CCM.   
- **Potential for inconsistencies with ALBC**: If users later decide to migrate to ALBC, there might be inconsistencies in how security groups are managed. Mitigation: Clearly document the limitations of this approach and the benefits of using ALBC for more advanced scenarios or a broader range of features.
- **Maintenance burden**: Maintaining this feature in CCM might become challenging if the upstream cloud-provider-aws project evolves significantly in its load balancer management. Mitigation: Aim to upstream the changes to benefit from community maintenance and consider the long-term strategy around load balancer controllers in OpenShift.
- **IAM Permissions(?)**: Incorrectly configured IAM permissions for the CCM could lead to failures in creating or managing security groups. Mitigation: Provide clear documentation on the necessary IAM permissions and potentially include checks in the CCM to verify these permissions.


### Drawbacks

> TODO/TBR

- The short-term solution requires engineering effort to implement and stabilize the changes in CCM and other components.
- Depending on the amount of changes in CCM, it will require Red Hat engineering commitment to maintain this functionality, especially if it's not fully upstreamed.
- This approach duplicates some functionality that already exists in the AWS Load Balancer Controller.

## Alternatives (Not Implemented)

> TODO/TBD

- **Defaulting to AWS Load Balancer Controller (ALBC) for the default router**: While ALBC provides more comprehensive support for NLB security groups , this option was deemed out of scope for the initial goal of minimal changes and addressing immediate customer needs within the existing CCM framework. Migrating to ALBC would involve more significant architectural changes and potentially impact existing deployments. However, this remains a viable long-term strategy .
- **Day-2 operations to switch the default router to use ALBC/LBC**: This would require users to manually deploy and configure ALBC after cluster installation, which does not meet the requirement for an opt-in during initial cluster deployment.

## Open Questions [optional]

> WIP/TODO

## Test Plan

> WIP/TODO

**cloud-provider-aws (CCM):**:

- e2e service Load Balancer type NLB with Security Groups (SG) needs to be implemented in the CCM component (upstream)

- Implement e2e tests for service Load Balancer type NLB with the service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true" annotation. These tests should verify:
  - The creation of an NLB with a newly created and associated Security Group.
  - The correct configuration of Ingress rules in the Security Group based on the service ports and loadBalancerSourceRanges.
  - The configuration of Egress rules, ensuring they are appropriately restricted.
  - The deletion of the Security Group when the service is deleted.
  - The default NLB provisioning behavior when the annotation is absent.
  - The behavior when the annotation is set to true on a non-NLB service type (should result in a warning or error).

**CIO**:

- Implement e2e tests to verify that the CIO correctly sets the service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true" annotation on the default router Service when the corresponding flag is enabled in the IngressController spec.

**installer**:

- Implement a job (dedicated or as part of existing e2e suites) to exercise the e2e flow of enabling Security Group management for NLB during cluster installation by setting the platform.aws.ingressController.managedSecurityGroup option in install-config.yaml.

**API**:

- Unit tests for the new boolean field managedSecurityGroup in the IngressController CRD.

## Graduation Criteria

> TODO/TBD: depends on the options.


### Dev Preview -> Tech Preview

> TODO/TBD: depends on the options.

### Tech Preview -> GA

> TODO/TBD: depends on the options.

### Removing a deprecated feature

> TODO/TBD: depends on the options.

## Upgrade / Downgrade Strategy

> TODO/TBD: depends on the options.

TBD:

The impact of enabling/disabling this feature on existing clusters needs to be considered. Upgrading a cluster with this feature enabled should not cause issues (?).

Downgrading might require manual cleanup of the managed Security Groups if the older version of the CCM does not support the annotation (need to review, not sure if we support downgrade).

## Version Skew Strategy

> TODO/TBD: depends on the options.

## Operational Aspects of API Extensions

> TODO/TBD: depends on the options.

- Monitoring of the CCM for errors related to Security Group creation, rule application, and deletion. Metrics around the number of managed security groups could also be useful.
- Logging in the CCM to track the status of Security Group operations, including creation, updates, and deletions. Include relevant AWS API call details in the logs for debugging.
- Potential for rate limiting on AWS API calls for Security Group management. Implement appropriate backoff and retry mechanisms in the CCM.
- Alerting on failures to create or manage security groups.

## Support Procedures

> TODO/TBD: depends on the options.

Document troubleshooting steps for issues related to NLB Security Group management, including:
- Checking the CCM logs for errors.
- Verifying the presence and configuration of the managed Security Group in the AWS console.
- Ensuring the service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true" annotation is present on the Service.
- Checking the IAM permissions of the CCM's service account.
- Providing guidance on common misconfigurations and their resolutions.

## Infrastructure Needed [optional]

> TODO/TBD: depends on the options.

(Double check: probably those permissions is already granted as CLB support manages SG, keeping it here just to review later)

Additional AWS permissions will be required for the CCM's IAM role to create, modify, and delete Security Groups. Specifically, the role will need permissions for ec2:CreateSecurityGroup, ec2:DescribeSecurityGroups, ec2:DeleteSecurityGroup, ec2:AuthorizeSecurityGroupIngress, ec2:AuthorizeSecurityGroupEgress, ec2:RevokeSecurityGroupIngress, and ec2:RevokeSecurityGroupEgress.

This needs to be clearly documented as a prerequisite for enabling this feature.

