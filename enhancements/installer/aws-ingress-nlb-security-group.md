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

Support for Security Groups on Network Load Balancers (NLB) through default Ingress when deploying an OpenShift cluster on AWS.


## Motivation

> WIP

Customers want to maintain a similar security configuration as Classic Load Balancers (CLB), with a security group attached, when deploying an OpenShift cluster on AWS with Network Load Balancers (NLB) in the default router.

The load balancer type used by the default router can be changed during installation by enabling it in the `install-config.yaml`, instructing the IngressController to create a service type LoadBalancer NLB. The Cloud Controller Manager (CCM) will provision an AWS Load Balancer type NLB without a security group.

AWS announced [1] support for Security Groups when deploying a Network Load Balancer recently, and the CCM for AWS (cloud-provider-aws) does not implement the feature. Furthermore, new features for service LoadBalancer are advised to be added to a dedicated controller, AWS Load Balancer Controller (LBC), which already supports deploying security groups for NLB.

Using a Network Load Balancer is a recommended network-based Load Balancer, and attaching a Security Group to an NLB is a security best practice. NLBs also do not support attaching security groups when they are not created with one.

[aws-lbc]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/

TODO: Evaluate the long-term improvements:
- Deploy LBC by IPI (suggestion 4.21+?).
- Default ingress to use NLB (suggestion 4.23+?).
- Use AWS Load Balancer Controller v2.5+ as the default controller for service-type Load Balancer. NOTE: This will deprecate the support of CLB (which has not been advised for years). (suggestion 4.25+?).

### User Stories

> WIP

Focus on customer (short-term):

- As an OpenShift administrator, I want to deploy a cluster on AWS using a Network Load Balancer with Security Groups in the default router service, so that I can comply with AWS best practices and "security findings"[1].

Focus on product improvement (future to unblock more features):

- As an OpenShift Engineer, I want to use NLB as the default LB for the router, following AWS best practices.

[1] TODO: "Security Findings" need to be expanded to collect exact examples. This comes from the customer's comment: https://issues.redhat.com/browse/RFE-5440?focusedId=25761057&page=com.atlassian.jira.plugin.system.issuetabpanels:comment-tabpanel#comment-25761057s


### Goals

> WIP

Topologies/deployment:
- Self-Managed: HA, Compact, TNA, SNO
- ROSA: Classic and HCP

#### Opt-in NLB provisioning with Security Groups for Default Ingress with FULL SG control by CCM <a name="goal-option-2"></a>

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) by setting a configuration in the `install-config.yaml`. CIO creates the service manifest with new annotation to signalize CCM to create Security Group, and CCM-AWS fully manages the SG lifecycle (similar to CLB, but as opt-in through annotation).

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB.
- Focus on customer scalability when enabling SG on NLB.
- Moderate changes to CCM.

T-Shirt Sizing/complexity by component:

| Component | T-Shirt Size | Complexity | Note                                                               |
|-----------|--------------|------------|--------------------------------------------------------------------|
| CCM       | M            | M          | API introduces annotation to "create SG on NLB" (default for CLB). |
| CIO       | S            | S          | API adds SG ID/Name to service annotation.                         |
| Installer | S            | S          | No SG management; API enabling feature.                            |
| ROSA CL   | S?           | S?         | No SG management; updates `install-config`.                        |
| ROSA HCP  | S?           | S?         | No SG management; creates CIO manifests to "enable NLB with SG".   |
| Day-2     | S            | S          | Patch CIO to recreate NLB.                                         |

Risk:
- CCM/upstream:
  - SG management increases controller complexity and scenarios to validate.
  - API changes.
  - More changes upstream increase the consensus/approvals required, especially for new features in service LB on CCM (prefer ALBC).
  - More changes in CCM to create and manage the SG lifecycle.
- CCM/downstream:
  - More complex to maintain when not upstreamed.

Day-2 update:
- Self-managed: Patch CIO.
- Managed Services: Patch CIO.
- Updates trigger service recreation of NLB.

Open questions:
- Considering this pattern is prefereble for security reasons, do we need a plan to enable this flow by default when using NLB?
- Can we start deploying self-managed by default IPI with NLB? Is there blocking considering NLB is already a default flow? Classic is discontinued (not recommended to be used by AWS) from a long time.

e2e PoC: https://github.com/openshift/installer/pull/9681

Proposed Phases:

**Phase 1: Create support on Self-Managed**

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the `install-config.yaml`.

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB.
- Focus on customer scalability when enabling SG on NLB.
- Minimal changes to CCM.

Goals:
- Installer manifest stage: installwe updates the CIO manifests when SG is enabled with NLB, setting the "security group enabled" flag in the CIO manifest.
- Cluster-Ingress-Operator:
  - API to receive the Security Group managed flag in the NLB parameters.
  - Controller creates the Service with annotation `service.beta.kubernetes.io/aws-load-balancer-manage-security-group:true`.
- Cloud Controller Manager (CCM):
  - Enable support of new annotation `service.beta.kubernetes.io/aws-load-balancer-manage-security-group:true` on creating service type Load Balancer NLB.
  - Creates the Security Group instance, Ingress and Egress rules based in the NLB Listeners and Target Groups' ports
  - Deletes the Security Group when the service is deleted
  - Enhance tests for Load Balancer component

**Phase 2: Create support on ROSA Classic**

Goals:
- TBD: Make ROSA Classic (Hive?) reads/reacts with new options in `install-config`: `platform.aws.ingressController.SecurityGroupEnabled`.

**Phase 3: Create support on ROSA HCP**

Goals:
- TBD: Hypershift sets the Service Annotation of Security Group when launching CIO.
- TBD: HCP flow must explored to validate if the self-managed proposed covers it.

___

### Non-Goals

> WIP

- Migrate to use ALBC as the default on CIO (See more in Alternatives).
- Use NLB as the default service type LoadBalancer by CCM.
- Synchronize all NLB features from ALBC to CCM.
- Change the existing CCM flow when deploying NLB .
- Change the default OpenShift install flow when deploying the default router using IPI (do we need to plan for that?).


## Proposal

> WIP/TODO


### Workflow Description

> WIP

#### OpenShift Self-managed

- Installer:
  - Create `install-config.yaml` enabling the use of Security Group, example `platform.aws.ingressController.securityGroupEnabled`, **when** `lbType=NLB` (already exists).
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
  - The installer generates the CIO manifests: enabling LB type NLB, and flag to enable Security Group.
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
  clientTLS:
    clientCA:
      name: ""
    clientCertificatePolicy: ""
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
  - CIO creates the Service instance for the default router, filling a new annotation telling CCM to manage SGs.
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
    service.beta.kubernetes.io/aws-load-balancer-managed-security-group: "true"
[...]
```
- Cloud Controller Manager (CCM):
  - CCM validates the annotation to manage SG on NLB, creates the SG and rules, and pass the SG ID to LB creation.
  - The annotation `service.beta.kubernetes.io/aws-load-balancer-managed-security-group` (new) msut be set to `true`, then creates the SG with required rules for ingress (based on listeners) and egress (based on service and health check ports).
  - CCM LB controller manages the SG lifecycle (controllers may exists in CLB).


#### OpenShift Managed (TBD)

- ROSA Classic:
    - need to ensure install-config.yaml option writes the CIO manifests enabling NLB with SGs.

- ROSA HCP:
    - need to ensure install-config.yaml option writes the CIO manifests enabling NLB with SGs.


### API Extensions

> WIP/TODO

#### AWS Cloud Controller Manager (CCM)

#### Cluster Ingress Operator (CIO)

- FeatureGate TP
- Receive an flag to enable Security Groups on Network Load Balancer structure

#### Installer

- "Enable" Security Group on install-config path platform.aws.ingressController.EnableSecurityGroup - only when deploying platform.aws.lbType=NLB

#### ROSA Classic

- TBD the e2e flow

#### Hypershift/ROSA HCP

- TBD the e2e flow

### Topology Considerations

#### Hypershift / Hosted Control Planes

> TODO/TBD

#### Standalone Clusters

<!-- Is the change relevant for standalone clusters? -->

All changes is proposed initially and exclusively for Standalone clusters.


#### Single-node Deployments or MicroShift

> TODO/TBD


### Implementation Details/Notes/Constraints

> TODO/TBD

### Risks and Mitigations

> TODO/TBD

### Drawbacks

> TODO/TBD

- the short-term would require more engineering effort to stabilize the 
- depending the amount of changes in CCM, it will require more Red Hat engineering commmitment to maitain CCM

## Alternatives (Not Implemented)

> TODO/TBD

- Day-2 operations to use default router using ALBO/LBC (is it supported?)
- 

## Open Questions [optional]

> WIP/TODO

1. [Proposal 1](TBD) resolves customer requirement, and 3 benefits the product in long-term. Would 1+3 a viable approach ?
2. Is a long-term solution deprecating service-LB on CCM (deprecating CLB) viable? ALB and NLB provides the same capabilities of CLB, and AWS keeps advocating migration paths as this product is stuck on time over the [last decade (~9 years](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/DocumentHistory.html)). Last feature [annouced in 2017](https://aws.amazon.com/about-aws/whats-new/2017/07/elastic-load-balancing-support-for-lcu-metrics-on-classic-load-balancer/).

## Test Plan

> WIP/TODO

**cloud-provider-aws**:

- e2e service Load Balancer type NLB with Security Groups (SG) needs to be implemented in the CCM component (upstream)

**CIO**:

- e2e BYO SG with NLB need to be implemented in the CIO component

**installer**:

- job(dedicated?) exercising e2e enabling SG with NLB need to be implemented in the installer component

**API**:

- TBD

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

## Version Skew Strategy

> TODO/TBD: depends on the options.

## Operational Aspects of API Extensions

> TODO/TBD: depends on the options.

## Support Procedures

> TODO/TBD: depends on the options.

## Infrastructure Needed [optional]

> TODO/TBD: depends on the options.
