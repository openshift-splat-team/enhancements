---
title: support-security-group-default-router
authors:
  - TBD
reviewers: # Include a comment about what domain expertise a reviewer is expected to bring and what area of the enhancement you expect them to focus on. For example: - "@networkguru, for networking aspects, please look at IP bootstrapping aspect"
  - @rvanderpool
approvers: # A single approver is preferred, the role of the approver is to raise important questions, help ensure the enhancement receives reviews from all applicable areas/SMEs, and determine when consensus is achieved such that the EP can move forward to implementation.  Having multiple approvers makes it difficult to determine who is responsible for the actual approval.
  - @patrick   # Installer changes
  - @joelspeed # API and CCM changes
  - @miciah    # CIO changes

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

FOCUS on customer (short-term):

- As an OpenShift administrator, I want to deploy a cluster on AWS using a Network Load Balancer with Security Groups in the default router service, so that I can comply with AWS best practices and "security findings"[1].

FOCUS on product improvement (future, nice-to-have, and to unblock more features):

- As an OpenShift Engineer, I want to use NLB as the default LB for the router, following AWS best practices.

[1] TODO: "Security Findings" need to be expanded to collect exact examples. This comes from the customer's comment: https://issues.redhat.com/browse/RFE-5440?focusedId=25761057&page=com.atlassian.jira.plugin.system.issuetabpanels:comment-tabpanel#comment-25761057s


### Goals

> WIP

Topologies/deployment:
- Self-Managed: HA, Compact, TNA, SNO
- ROSA: Classic and HCP

#### Option 1. Opt-in NLB provisioning with Security Groups for Default Ingress with NO SG control by CCM

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the `install-config.yaml`. The installation agent (`openshift-install`, `hypershift`) manages the Security Group creation.

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB.
- Focus on customer scalability when enabling SG on NLB.
- Minimal changes to CCM.

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Size | Complexity | Note |
| -- | -- | -- | -- |
| CCM   | S | S | No API changes, No SG management, Opt-in. |
| CIO | S | S | API adds SG ID/Name to service annotation. |
| Installer | S | S | API enabling feature; Creates Ingress SG (SDK). |
| ROSA CL | M? | M? | TBD: API enabling feature(?); creates Ingress SG; updates `install-config`. |
| ROSA HCP | M? | M? | TBD: API enabling feature(?); SG mgt; creates CIO manifests to enable SG. |
| Day-2 | S | M | BYO SG (can managed services automate through CLI?), patch CIO to recreate NLB. |

Risk:
- Upstream CCM changes can take longer than expected (small changes may propagate downstream).
- Default Ingress changes to ports 80 and 443 (if they occur) require manual SG rule updates.

Day-2 update:
- Self-managed: Requires the user to create SG and patch CIO.
- Managed: Requires creating SG and patching CIO (can be automated via CLI).
- Updates trigger service recreation of NLB.

e2e PoC: https://github.com/openshift/installer/pull/9681

**Phase 1: Create support on Self-Managed**

Goals:
- Installer manifest stage: When SG is enabled, set the security group names in the CIO manifest.
- Installer infra stage: When SG is enabled, create the security group and rules for ingress (InfraReady hook).
- Cluster-Ingress-Operator:
  - API to receive the Security Group in the NLB parameters.
  - Service Controller creates the annotation `service.beta.kubernetes.io/aws-load-balancer-security-groups` with the custom Security Group.
- Enable support for BYO (unmanaged) Security Groups for AWS Cloud Controller Manager (CCM/cloud-provider-aws).

**Phase 2: Create support on ROSA Classic**

Goals:
- TBD: How ROSA can enable the option in `install-config`: `platform.aws.ingressController.SecurityGroupEnabled`.

**Phase 3: Create support on ROSA HCP**

Goals:
- TBD: Hypershift creates the ingress Security Group for the ingress (after VPC creation).
- TBD: Hypershift sets the Security Group in CIO manifests.

---

#### Option 2. Opt-in NLB provisioning with Security Groups for Default Ingress with FULL SG control by CCM

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the `install-config.yaml`. CCM-AWS fully manages the SG lifecycle (similar to CLB, but as opt-in through annotation).

Highlights:
- Focus on short-term resolution of security issues when not using SG on NLB.
- Focus on customer scalability when enabling SG on NLB.
- Moderate changes to CCM.

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Size | Complexity | Note |
| -- | -- | -- | -- |
| CCM | M | M | API introduces annotation to "create SG on NLB" (default for CLB). |
| CIO | S | S | API adds SG ID/Name to service annotation. |
| Installer | S | S | No SG mgt; API enabling feature. |
| ROSA CL | S? | S? | No SG mgt; updates `install-config`. |
| ROSA HCP | S? | S? | No SG mgt; creates CIO manifests to "enable NLB with SG". |
| Day-2 | S | S | Patch CIO to recreate NLB. |

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


e2e PoC: N/A

---

#### Option 3. NLB feature parity on CCM with ALBC

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Size | Complexity | Note |
| -- | -- | -- | -- |
| CCM | XXL | XXL | NLB feature parity plan with ALBC. Long-term commitment and support by RH. |
| CIO | S | S | API adds SG ID/Name to service annotation. |
| Installer | S | S | No SG mgt; API enabling feature. |
| ROSA CL | S? | S? | No SG mgt; updates `install-config`. |
| ROSA HCP | S? | S? | No SG mgt; creates CIO manifests to "enable NLB with SG". |
| Day-2 | S | S | Patch CIO to recreate NLB. |


e2e PoC: N/A

---

#### Option 4. CIO switches to ALBC

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Size | Complexity | Note |
| -- | -- | -- | -- |
| CCM | - | - | CCM will not be used by the default router. |
| CIO | XXL | XL | API: (short-term) opt-in NLB provisioning with SG using ALBC; (long-term) all new provisioning with ALBC; move images to payload; manage operator lifecycle (permissions, etc.). |
| Installer | S | S | No SG mgt; API enabling feature. |
| ROSA CL | S? | S? | No SG mgt; updates `install-config`. |
| ROSA HCP | S? | S? | No SG mgt; creates CIO manifests to "enable NLB with SG". |
| Day-2 | S | S | Patch CIO to recreate NLB. |


e2e PoC: N/A

---

#### Option 1+4. Short-term security fixes with long-term Ingress improvements

> Note: Can be 2+4 too, but this will impact resource investments in feature parity with CCM.

In the short term, support NLB provisioning with Security Groups as opt-in by CCM, resolving security issues, and achieve long-term modernization by inheriting features added in the upstream ALBC project, which is actively maintained by AWS.

Highlights:
- Resolves short-term security issues when not using SG on NLB.
- Improves customer scalability when enabling SG on NLB.
- Moderate changes to CCM.
- Provides a long-term plan to modernize Load Balancer features by inheriting updates from the controller in the upstream project maintained by the community (and AWS).

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Size | Complexity | Note |
| -- | -- | -- | -- |
| CCM | S | S | No API changes, No SG management, Opt-in. |
| CIO | XXL | XL | API: (short-term) opt-in NLB provisioning with SG using ALBC; (long-term) all new provisioning with ALBC; move images to payload; manage operator lifecycle (permissions, etc.). |
| Installer | S | S | API enabling feature; Creates Ingress SG (SDK). |
| ROSA CL | M? | M? | TBD: (short-term) API enabling feature(?); creates Ingress SG; updates `install-config`; (long-term) fixes many issues. |
| ROSA HCP | M? | M? | TBD: (short-term) API enabling feature(?); SG mgt; creates CIO manifests to enable SG; (long-term) fixes many issues. |
| Day-2 | S | M | (short-term) BYO SG (can managed services automate through CLI?), (short-term & long-term) patch CIO to recreate NLB. |


e2e PoC: N/A

---

### Non-Goals

> WIP

Short-term:

  - Migrate to use ALBC as the default on CIO.
  - Use NLB as the default service type LoadBalancer.
  - Synchronize NLB features from LBC to CCM.
  - Change the current CCM flow when deploying NLB.
  - Change the current OpenShift e2e flow when deploying the default router using IPI.

## Proposal

> WIP/TODO


### Workflow Description

> WIP

#### Option 1. Workflow

- Create `install-config.yaml` enabling the use of Security Group **and** `lbType=NLB` (already exists).
- The installer creates a security group to be used by the ingress controller during the InfraReady phase.
- The installer generates the CIO manifests: enabling LB type NLB passing Security Group Names (IDs are not known yet during the manifest phase).
- CIO creates the service for the default router, filling in the annotations for NLB and SG Names.
- CCM checks annotations, maps Names to IDs, and provisions the Load Balancer NLB with the security group, updating SGs with the required rules for ingress (based on listeners) and egress (based on service and health check ports).


#### Option 2. Workflow CCM Manage SG

- Create `install-config.yaml` enabling the use of Security Group **and** `lbType=NLB` (already exists).
- The installer generates the CIO manifests: enabling LB type NLB.
- CIO creates the service for the default router, filling a new annotation telling CCM to manage SGs.
- CCM checks annotation to manage SG on NLB, creates the SG and rules, and pass the SG ID to LB creation. CCM controllers manages the SG lifecycle (controllers may exists in CLB).


### API Extensions

> WIP/TODO

CIO:

- FeatureGate TP
- Receipt SG list on CIO

Installer:

- "Enable" SG on install-config - only when deploying lbType=NLB

ROSA Classic:

- TBD. Is there any?

Hypershift/ROSA HCP:

- TBD

### Topology Considerations

#### Hypershift / Hosted Control Planes

> WIP/TODO

#### Standalone Clusters

<!-- Is the change relevant for standalone clusters? -->

All changes is proposed initially and exclusively for Standalone clusters.


#### Single-node Deployments or MicroShift

> WIP/TODO


### Implementation Details/Notes/Constraints

> WIP/TODO

### Risks and Mitigations

> WIP/TODO

### Drawbacks

> WIP/TODO

- the short-term would require more engineering effort to stabilize the 
- depending the amount of changes in CCM, it will require more Red Hat engineering commmitment to maitain CCM

## Alternatives (Not Implemented)

> WIP/TODO

- Day-2 operations to use default router using ALBO/LBC (is it supported?)
- 

## Open Questions [optional]

> WIP/TODO

1. [Proposal 1](TBD) resolves customer requirement, and 3 benefits the product in long-term. Would 1+3 a viable approach ?
2. Is a long-term solution deprecating service-LB on CCM (deprecating CLB) viable? ALB and NLB provides the same capabilities of CLB, and AWS keeps advocating migration paths as this product is stuck on time over the [last decade (~9 years](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/DocumentHistory.html)). Last feature [annouced in 2017](https://aws.amazon.com/about-aws/whats-new/2017/07/elastic-load-balancing-support-for-lcu-metrics-on-classic-load-balancer/).

## Test Plan

> WIP/TODO

**machine-provider-aws**:

- e2e BYO SG with NLB need to be implemented in the CCM component

**CIO**:

- e2e BYO SG with NLB need to be implemented in the CIO component

**installer**:

- job exercising e2e enabling SG with NLB need to be implemented in the installer component

**API**:

- TBD

## Graduation Criteria

> TODO: depends on the options. TBD


### Dev Preview -> Tech Preview

> TODO: depends on the options. TBD

### Tech Preview -> GA

> TODO: depends on the options. TBD

### Removing a deprecated feature

> TODO: depends on the options. TBD

## Upgrade / Downgrade Strategy

> TODO: depends on the options. TBD

## Version Skew Strategy

> TODO: depends on the options. TBD

## Operational Aspects of API Extensions

> TODO: depends on the options. TBD

## Support Procedures

> TODO: depends on the options. TBD

## Infrastructure Needed [optional]

> TODO: depends on the options. TBD
