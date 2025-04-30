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

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the install-config.yaml. Installation agent (openshift-install, hypershift) manages the Security Group creation.

Hightlights:
- Focus on short-term resolving security issues when not using SG on NLB
- Focus on customer scalation of enabling SG on NLB
- Minimum changes on CCM

T-Shirt Sizing/complexity by component:
| Component | T-Shirt Sz | Complexity | Note |
| -- | -- | -- | -- |
| CCM   | S | S | No API changes, No SG management, Opt-in |
| CIO | S | S | API adding SG ID/Name to service Annotation |
| Installer | S | S | API enabling feature; Creates Ingress SG (SDK) |
| ROSA CL | M? | M? | TBD:API enabling feature(?); creates Ingress SG; update install-config |
| ROSA HCP | M? | M? | TBD:API enabling feature(?); SG mgr; create CIO manifests to enable SG |
| Day-2 | S | M | BYO SG (can managed svc automate through cli?), patch CIO to recreate NLB;  |


Risk:
- Upstream CCM changes can take longer than expected (Small change can carry dowstream)
- Default Ingress changes the port from 80 and 443(happen??) requires manual SG's rules updates

Day-2 update:
- Self-manage: require user to create SG and patch CIO
- Manage: require to create SG and patch CIO (can be automated by CLI)
- Updates trigger service recreating NLB

**Phase 1: Create support on Self-Managed**

Goals:
- Installer manifest stage: when SG is enabled, set the security group names to the CIO manifest
- Installer infra stage: when SG is enabled, create security group and rules for ingress (InfraReady hook)
- Cluster-Ingress-Operator:
  - API to receipt the Security Group in the NLB parameters
  - Service Controller create the annotation `service.beta.kubernetes.io/aws-load-balancer-security-groups` with custom Security Group
- Enable support of BYO (unmanaged) Security Groups for AWS Cloud Controller Manager (CCM/cloud-provider-aws)


**Phase 2: Create support on ROSA Classic**

Goals:
- TBD how ROSA can enable the option on install-config: `platform.aws.ingressController.SecurityGroupEnabled`

**Phase 3: Create support on ROSA HCP**

Goals:
- TBD hypershift creating ingress Security Group for the ingress (after VPC creation)
- TBD hypershift setting the Security Group in CIO manifests


#### Option 2. Opt-in NLB provisioning with Security Groups for Default Ingress with FULL SG control by CCM

Users will be able to deploy OpenShift on AWS with the default Ingress using Network Load Balancers with Security Groups when enabled (opt-in) in the install-config.yaml. CCM-AWS full manages the SG lifecycle (similar CLB, but as opt-in through annotation).

Hightlights:
- Focus on short-term resolving security issues when not using SG on NLB
- Focus on customer scalation of enabling SG on NLB
- Medium changes on CCM


T-Shirt Sizing/complexity by component:

| Component | T-Shirt Sz | Complexity | Note |
| -- | -- | -- | -- |
| CCM | M | M | API introducing annotation to "create SG on NLB"(default for CLB) |
| CIO | S | S | API adding SG ID/Name to service Annotation |
| Installer | S | S | no SG mgr; API enabling feature; |
| ROSA CL | S? | S? | no SG mgr; update install-config |
| ROSA HCP | S? | S? | no SG mgr; create CIO manifests to "enable NLB with SG" |
| Day-2 | S | S | patch CIO to recreate NLB |


Risk:
- CCM/upstream:
  - SG management increases controller complexity and scenarios to validate.
  - API changes 
  - More changes in upstream increases the consensus/approvals, specially new features in service LB on CCM (prefer ALBC)
  - More cchanges in CCM making it creates and manage SG lifecycle
- CCM/downstream:
  - more complex to carry  when not in upstream

Day-2 update:
- Self-managed: patch CIO
- Managed Services: patch CIO
- Updates trigger service recreating NLB


#### Option 3. NLB feature parity on CCM with ALBC

T-Shirt Sizing/complexity by component:

| Component | T-Shirt Sz | Complexity | Note |
| -- | -- | -- | -- |
| CCM | XXL | XXL | NLB Feature parity plan with ALBC. Long-term commitment and support by RH |
| CIO | S | S | API adding SG ID/Name to service Annotation |
| Installer | S | S | no SG mgr; API enabling feature; |
| ROSA CL | S? | S? | no SG mgr; update install-config |
| ROSA HCP | S? | S? | no SG mgr; create CIO manifests to "enable NLB with SG" |
| Day-2 | S | S | patch CIO to recreate NLB |


#### Option 4. CIO switches to ALBC

T-Shirt Sizing/complexity by component:

| Component | T-Shirt Sz | Complexity | Note |
| -- | -- | -- | -- |
| CCM | - | - | CCM will not be used by default router |
| CIO | XXL | XL | API: (short-term) opt-in NLB provisioning with SG using ALBC; long-term: all new provisioning with ALBC; move imgs to payload; manage oper lifecycle(perms, etc) |
| Installer | S | S | no SG mgr; API enabling feature; |
| ROSA CL | S? | S? | no SG mgr; update install-config |
| ROSA HCP | S? | S? | no SG mgr; create CIO manifests to "enable NLB with SG" |
| Day-2 | S | S | patch CIO to recreate NLB |


#### Option 1+4. Short-term security fixes with long-term Ingress improvements

> Note: can be 2+4 too, but will impact in the resource investiments in feature parity with CCM.

In short-term support NLB provisioning with Security Group as opt-in by CCM, resolving security issues,
and get long-term modernization by inheriting features added in upstream project ALBC which is actively maitained by AWS.

Hightlights:
- Resolves on short-term resolving security issues when not using SG on NLB
- Resolves customer scalation of enabling SG on NLB
- Medium changes on CCM
- Provide long-term plan to modernize Load Balancer features by inheriting updates from controller on upstream project maitained by Community (and AWS)


T-Shirt Sizing/complexity by component:

| Component | T-Shirt Sz | Complexity | Note |
| -- | -- | -- | -- |
| CCM | S | S | No API changes, No SG management, Opt-in |
| CIO | XXL | XL | API: (short-term): opt-in NLB provisioning with SG using ALBC; (long-term): all new provisioning with ALBC; move imgs to payload; manage oper lifecycle(perms, etc) |
| Installer | S | S | API enabling feature; Creates Ingress SG (SDK) |
| ROSA CL | M? | M? | TBD:(short): API enabling feature(?); creates Ingress SG; update install-config; (long):fix many issues |
| ROSA HCP | M? | M? | TBD: (short): API enabling feature(?); SG mgr; create CIO manifests to enable SG; (long):fix many issues | |
| Day-2 | S | M | (short): BYO SG (can managed automate through cli?), (short&long): patch CIO to recreate NLB; |



<!-- 
===> DRAFT/old notes:

#### Option 1. CCM configures the SG when deploying NLB. (Minimum support)

CCM enable SG when provisioning NLB only when the annotation XXX with SG ID is added.

Pros:
- Minor changes in CCM (upstream): BYO SG approach (similar [LBC annotation `alb.ingress.kubernetes.io/security-groups`](https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/ingress/annotations/#security-groups))
- SG is not created by CCM, and CCM is not adding rules (preserving LBC behavior for annotations security-groups and manage-backend-security-group-rules)
- Does not required API updates for CCM (right, @joel?)
- We can stick only the default ingress/router using that approach, requiring any new ingress to use LBC-ish (already supported)

Cons:
- Installer (or CIO) needs to create the SG, passing to the CCM with required rules
- BYO SG approach for CCM would require CIO managing SG rules when provisioning new ingress
    - or at least validate if exists
    - or not support this feature for other services than the default router, leading user to use ALBO/LBC


PROBLEM:
- installer can't create SG in the manifest phase, so it wont have IDs, only names (candidate)

#### Option 2. CCM manages the SG when deploying NLB. (may require API changes and more complexity)

Pros:
- Installer (or CIO) does not required to manage the SG
- Increase UX as users don't need to handle with SG in Day-2 when creating new ingress

Cons:

- Requires more complexity to manage SG when NLB on CCM (upstream). SG management logic already exists in the CLB controller
- May require API add/change to determine when SG will be enabled for NLB. Example of LBC annotation manage-backend-security-group-rules


#### Option 3. Use AWS LBC to deploy Ingress NLB. (more complex downstream, no changes upstream)

Changes:
- Requires installing ALBO from OLM during installation.

Pros:
- Unlocks new NLB features added to LBC for OpenShift ingress.

Cons:
- Defaulting to LBC may introduce unknown issues.
- Be careful.
- Create a clear migration path.

Open questions:

- Is there an advantage to using LBC as the default service type LB controller (disabling CCM service-LB capabilities)?
    - LBC does not support CLBs.
        - Pros: Improves UX as default service LBs will use the latest controller features for LB.
        - Cons: Breaking change for apps using default service config (with CLB).
    - AWS has advised against using CLBs for years. This will prepare OCP to follow cloud-provider best practices. It's important to have a clear deprecation path.

#### Option 1+3. Focus on "enabling" SG on NLB using BYO SG on CCM, and plan the migration to LBC

Laser focus on short-term customer issues and unlock OpenShift's long-term ingress features with NLB.

Pros:
- Delivers customer requirements ASAP.
- Reduces changes in upstream CCM.
- Enables OpenShift to consume features available on LBC.
- Prevents duplicate maintenance of CCM/LBC codebases.

Cons:
- ?

Phases for #1+3:

- Phase 1 (TP SGs for NLB using CCM BYO SG):
    - Patch CCM with required changes for BYO SG when using NLB.
    - Create APIs for BYO SG on CIO (NLB only).
    - Installer:
        - Add a configuration option in `install-config` to "use SG" when `lbType == NLB`.
        - Provision SG with expected rules (also adding those to nodes' SG).

- Phase 2 (GA SGs for NLB, Dev Preview LBC through CIO):
    - Enable CIO provisioning using LBC/ALBO when using NLB.
    - CIO API to enable LBC service provisioning (e.g., manage-backend-security-group-rules).
        - Retain API added in Phase 1 for future BYO SG use (user-facing, exposed in `install-config`).
    - Installer:
        - Add a configuration option in `install-config` to opt-in ingress provisioning with LBC. (or make the flag "Use SG when NLB" to trigger the LBC)
        - When enabled, do not provision SGs by installer (delegate to CIO->LBC).
    - Add a validation webhook to inform users about Classic Load Balancer (CLB) desconuation(recommendation to use NLB) when creating type LB services (non-NLB).
        - Alternatively, provide an informational message advising users to use ingress provisioned with CIO, which uses NLBs, to follow best practices and prevent future deprecations.
    - Existing NLBs managed by CCM:
      - How to make controllers aware of this? Can we live with two controllers, making the controllers to know that the NLB has been provisioned by each one.

- Phase 3 (TP LBC on CIO, Dev Preview NLB by default CIO):):
    - Installer:
        - Default to provisioning CIO services with LBC (defaults to lbType==NLB <- dev preview this?).
        - Customers can use CLB as opt-in 

- Phase 4 (GA LBC on CIO, TP NLB by default CIO):
    - Installer:
        - Default to provisioning CIO services with LBC (defaults to lbType==NLB <- TP this?).

- Phase 5: (GA )
    - Make AWS LBC the default controller for service type LoadBalancer.
    - Classic Load Balancer is no longer available for CIO (only when customers wants to  create service-LB directly)
 -->

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
