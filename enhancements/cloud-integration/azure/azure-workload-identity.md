---
title: azure-workload-identity
authors:
  - abutcher
reviewers: # Include a comment about what domain expertise a reviewer is expected to bring and what area of the enhancement you expect them to focus on. For example: - "@networkguru, for networking aspects, please look at IP bootstrapping aspect"
  - "@2uasimojo"
  - "@derekwaynecarr, for overall architecture."
  - "@sdodson, for overall architecture."
  - "@jharrington22, for service delivery considerations."
  - "@RomanBednar, for azure file/disk operators."
  - "@joelspeed, for MAPI / machine api operator."
  - "@dmage, for image registry operator, please look at resource group being removed from credential secret and lookup from infrastructure object."
  - "@Miciah, for ingress operator."
  - "@patrickdillon, for installer."
  - "@abhat, for cncc + cloud network operator."

approvers:
  - "@sdodson"
  - "@deads2k"
  - "@jharrington22"
api-approvers: # In case of new or modified APIs or API extensions (CRDs, aggregated apiservers, webhooks, finalizers). If there is no API change, use "None"
  - None
creation-date: 2022-12-08
last-updated: 2023-03-29
tracking-link:
  - https://issues.redhat.com/browse/CCO-187
see-also:
  - "enhancements/cloud-integration/aws/aws-pod-identity.md"
replaces:
  - ""
superseded-by:
  - ""
---

# Azure Workload Identity

## Summary

Core OpenShift operators (e.g. ingress, image-registry, machine-api) use long-lived credentials to access Azure API services today. This enhancement proposes an implementation by which OpenShift operators would utilize short-lived, [bound service account tokens](https://docs.openshift.com/container-platform/4.11/authentication/bound-service-account-tokens.html) signed by OpenShift that can be
trusted by Azure as the `ServiceAccounts` have been associated with [Azure Managed Identities](https://learn.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview). [Workload identity federation support for Managed Identities](https://github.com/Azure/azure-workload-identity/issues/325) was recently made public preview by Azure
([announcement](https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview)) and is the basis for this proposal.

## Motivation

Previous enhancements have implemented short-lived credential support via [STS for AWS](https://github.com/openshift/enhancements/pull/260) and GCP Workload Identity. This enhancement proposal intends to complement those implementations within the Azure platform.

### User Stories

- As a cluster-creator, I want to create a self-managed OpenShift cluster on Azure that utilizes short-lived credentials for core operator authentication to Azure API services so that long-lived credentials do not live on the cluster.
- As a cluster-administrator, I want to provision Federated Managed Identities within Azure and use Federated Managed Identities for my own workload's authentication to Azure API services.

### Goals

- Core OpenShift operators utilize short-lived, bound service account token credentials to authenticate with Azure API Services.
- Self-managed OpenShift administrators can create Azure infrastructure necessary to utilize workload identity federation such as an Azure blob storage container based OIDC using `ccoctl`.
- Self-managed OpenShift administrators can create Azure Managed Identities via `ccoctl`'s processing of `CredentialsRequest` custom resources extracted from the release image prior to installation and provide the secrets output as manifests for installation which serve as the credentials for core OpenShift operators.
- A user can utilize a Federated Azure Managed Identity Credential for their workload using the [mutating admission webhook provided by Azure Workload Identity](https://azure.github.io/azure-workload-identity/docs/installation/mutating-admission-webhook.html).

### Non-Goals

- Creation of Azure Managed Identity infrastructure (OIDC, managed identities, federated credentials) in managed environments (eg. ARO).
- Role granularity for the explicit necessary permissions granted to
  Managed Identities. Permissions needed by operator identities are
  enumerated within `CredentialsRequests` for platforms such as AWS,
  example:
  [aws-ebs-csi-driver-operator](https://github.com/openshift/cluster-storage-operator/blob/f1ddb697afb3c33d6d45936e58fad101abe26f13/manifests/03_credentials_request_aws.yaml). Granular
  permissions for operators on Azure are not a goal of this
  enhancement but should be implemented either in parallel to this
  enhancement or as a followup.

## Proposal

In this proposal, the Cloud Credential Operator's command-line utility (`ccoctl`) will be extended with subcommands for Azure which will provide methods for generating the Azure infrastructure (blob container OIDC, managed identities and federated credentials) and secret manifests necessary to create an Azure cluster that utilizes Azure Workload Identity for core OpenShift operator authentication.

OpenShift operators will be updated to create Azure clients using a bound `ServiceAccount` token that has been associated with a Managed Identity (identified by `clientID`) in Azure. Operators or repositories that we expect will need changes, listed in [CCO-235](https://issues.redhat.com/browse/CCO-235):

- [cloud-credential-operator](https://github.com/openshift/cloud-credential-operator)
- [cluster-image-registry-operator](https://github.com/openshift/cluster-image-registry-operator)
- [cluster-ingress-operator](https://github.com/openshift/cluster-ingress-operator)
- [cluster-storage-operator](https://github.com/openshift/cluster-storage-operator)
- [machine-api-operator](https://github.com/openshift/machine-api-operator)
- [machine-api-provider-azure](https://github.com/openshift/machine-api-provider-azure)
- [docker-distribution](https://github.com/openshift/docker-distribution)
- [azure-disk-csi-driver-operator](https://github.com/openshift/azure-disk-csi-driver-operator)
- [azure-file-csi-driver-operator](https://github.com/openshift/azure-disk-csi-driver-operator)
- [cloud-controller-manager-operator](https://github.com/openshift/cluster-cloud-controller-manager-operator)
- [cloud-provider-azure](https://github.com/kubernetes-sigs/cloud-provider-azure/)
- [cluster-api-provider-azure](https://github.com/kubernetes-sigs/cluster-api-provider-azure)
- [cluster-network-operator](https://github.com/openshift/cluster-network-operator/)
- [cloud-network-config-controller](https://github.com/openshift/cloud-network-config-controller)

Managed Identity details such as the `clientID`, `tenantID` and path
to the mounted Service Account token necessary for creating a client
can also be supplied to pods as environment variables via a [mutating
admission webhook provided by Azure Workload
Identity](https://azure.github.io/azure-workload-identity/docs/installation/mutating-admission-webhook.html). This
webhook would be deployed and lifecycled by the Cloud Credential
Operator such that the webhook could be utilized to supply credential
details to user workloads. Core OpenShift operators will not rely on
the webhook.

### Workflow Description

#### Cloud Credential Operator Command-line Utility (ccoctl)

The Cloud Credential Operator's command-line utility (`ccoctl`) will be extended with subcommands for Azure which provide methods for,
- Generating a key pair to be used for `ServiceAccount` token signing for a fresh OpenShift cluster.
- Creating an Azure blob storage container to serve as the identity provider in which to publish OIDC and JWKS documents needed to establish trust at a publicly available address. This subcommand will output a modified cluster `Authentication` CR, containing a `serviceAccountIssuer` pointing to the Azure blob storage container's URL to be provided as a manifest for installation.
- Creating Managed Identity infrastructure with federated credentials for OpenShift operator `ServiceAccounts` (identified by namespace & name) and to output secrets containing the `clientID` of the Managed Identity to be provided as manifests for the installer. This command will process `CredentialsRequest` custom resources to identify service accounts that will be associated with Managed
  Identities in Azure as federated credentials. For self-managed installation, `CredentialsRequests` will be extracted from the release image.

```sh
$ ./ccoctl azure
Creating/updating/deleting cloud credentials objects for Azure

Usage:
  ccoctl azure [command]

Available Commands:
  create-all                Create key pair, identity provider and Azure Managed Identities
  create-identity-provider  Create identity provider
  create-key-pair           Create a key pair
  create-managed-identities Create Azure Managed Identities
  delete                    Delete Azure identity provider and Managed Identity infrastructure

Flags:
  -h, --help   help for azure

Use "ccoctl azure [command] --help" for more information about a command.
```

#### Azure CredentialsRequests ServiceAccounts

`CredentialsRequests` for the Azure platform must now list
[ServiceAccountNames](https://github.com/openshift/cloud-credential-operator/blob/1f7a2602bf8a9ddec5d8fc29f77215697d9e7c07/pkg/apis/cloudcredential/v1/types_credentialsrequest.go#L57-L62)
in order to for `ccoctl` to be able to create federated credentials
for an Azure Managed Identity that are associated with the `name` and
`namespace` of the `ServiceAccount`. Example:
[aws-ebs-csi-driver-operator](https://github.com/openshift/cluster-storage-operator/blob/f1ddb697afb3c33d6d45936e58fad101abe26f13/manifests/03_credentials_request_aws.yaml#L11-L13).

#### Credentials secret

OpenShift operators currently obtain their long-lived credentials from a config secret with the following format:

```yaml
apiVersion: v1
data:
  azure_client_id: <client id>
  azure_client_secret: <client secret>
  azure_region: <region>
  azure_resource_prefix: <resource group prefix eg. "abutcher-az-t68n4">
  azure_resourcegroup: <resource group eg. "abutcher-az-t68n4-rg">
  azure_subscription_id: <subscription id>
  azure_tenant_id: <tenant id>
kind: Secret
type: Opaque
```

We propose that when utilizing Azure Workload Identity, the credentials secret will contain an `azure_client_id` that is the `clientID` of the Managed Identity provisioned by `ccoctl` for the operator. The `azure_client_secret` key will be absent and instead we can provide the path to the mounted `ServiceAccount` token as an `azure_federated_token_file` key; the path to the mounted token is well
known and is specified in the operator deployment.

The resource group in which the installer will create infrastructure will not be known when these secrets are generated by `ccoctl` ahead of installation and operators which rely on `azure_resourcegroup` and `azure_resource_prefix` such as the
[image-registry](https://github.com/openshift/cluster-image-registry-operator/blob/8556fd48027f89e19daad36e280b60eb93d012d4/pkg/storage/azure/azure.go#L95-L100) should obtain the resource group details from the cluster `Infrastructure` object instead.

```yaml
apiVersion: v1
data:
  azure_client_id: <client id>
  azure_federated_token_file: <path to mounted service account token, eg. "/var/run/secrets/openshift/serviceaccount/token">
  azure_region: <region>
  azure_subscription_id: <subscription id>
  azure_tenant_id: <tenant id>
kind: Secret
type: Opaque
```

#### Creating workload identity clients in operators

In order to create Azure clients which utilize a `ClientAssertionCredential`, operators must update to version `>= v1.3.0-beta.4` of the azidentity package within [azure-sdk-for-go](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.3.0-beta.4). Ahead of this work, due to the [end of life
announcement](https://techcommunity.microsoft.com/t5/microsoft-entra-azure-ad-blog/microsoft-entra-change-announcements-september-2022-train/ba-p/2967454) of the Azure Active Directory Authentication Library (ADAL), PRs (ex. [openshift/cluster-ingress-operator](https://github.com/openshift/cluster-ingress-operator/pull/846)) have been opened for operators to migrate to creating clients via
azidentity which are converted into an authorizer for use with v1 clients. Once these changes have been made, we propose that OpenShift operators continue to utilize a config secret to obtain authentication details as described in the previous section but create workload identity clients when the `azure_client_secret` is absent and when  `azure_federated_token_file` fields are found in the
config. Config secrets will be generated by cluster creators prior to installation by using `ccoctl` and will be provided as manifests for install.

Due to the deployment of the Azure Workload Identity mutating admission webhook, environment variables should also be respected by client instantiation as an alternative way of supplying the `clientID` eg. `AZURE_CLIENT_ID`, `tenantID` eg. `AZURE_TENANT_ID` and `federatedTokenFile` eg. `AZURE_FEDERATED_TOKEN_FILE`.

Code sample ([commit](https://github.com/openshift/cluster-ingress-operator/compare/master...jstuever:cluster-ingress-operator:cco-318
)) taken from a [proof of concept](https://gist.github.com/abutcher/2a92d678a6da98d5c98a188aededab69) based on [openshift/cluster-ingress-operator](https://github.com/openshift/cluster-ingress-operator/pull/846):

All operators would need code changes similar to the sample below, introducing `azidentity.NewWorkloadIdentityCredential()` for procuring a credential as an alternative to `azidentity.NewClientSecretCredential()` for the current config secret.

```go
func getAuthorizerForResource(config Config) (autorest.Authorizer, error) {
    ...

	// Fallback to using tenant ID from env variable if not set.
	if strings.TrimSpace(config.TenantID) == "" {
		config.TenantID = os.Getenv("AZURE_TENANT_ID")
		if strings.TrimSpace(config.TenantID) == "" {
			return nil, errors.New("empty tenant ID")
		}
	}

	// Fallback to using client ID from env variable if not set.
	if strings.TrimSpace(config.ClientID) == "" {
		config.ClientID = os.Getenv("AZURE_CLIENT_ID")
		if strings.TrimSpace(config.ClientID) == "" {
			return nil, errors.New("empty client ID")
		}
	}

	// Fallback to using client secret from env variable if not set.
	if strings.TrimSpace(config.ClientSecret) == "" {
		config.ClientID = os.Getenv("AZURE_CLIENT_SECRET")
		// Skip validation; fallback to token (below) if env variable is also not set.
	}

	var (
		cred azcore.TokenCredential
		err  error
	)
	if strings.TrimSpace(config.ClientSecret) == "" {
		options := azidentity.WorkloadIdentityCredentialOptions{
			ClientOptions: azcore.ClientOptions{
				Cloud: cloudConfig,
			},
		}
		cred, err = azidentity.NewWorkloadIdentityCredential(config.TenantID, config.ClientID, "/var/run/secrets/openshift/serviceaccount/token", &options)
		if err != nil {
			return nil, err
		}
	} else {
		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: azcore.ClientOptions{
				Cloud: cloudConfig,
			},
		}
		cred, err = azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, &options)
		if err != nil {
			return nil, err
		}
	}

	scope := endpointToScope(config.Environment.TokenAudience)

	// Use an adapter so azidentity in the Azure SDK can be used as
	// Authorizer when calling the Azure Management Packages, which we
	// currently use. Once the Azure SDK clients (found in /sdk) move to
	// stable, we can update our clients and they will be able to use the
	// creds directly without the authorizer. The schedule is here:
	// https://azure.github.io/azure-sdk/releases/latest/index.html#go
	authorizer := azidext.NewTokenCredentialAdapter(cred, []string{scope})

	return authorizer, nil
}
```

#### Mutating admission webhook

CCO will also deploy and lifecycle the [Azure Workload Identity mutating admission webhook](https://azure.github.io/azure-workload-identity/docs/installation/mutating-admission-webhook.html) on Azure clusters such that user workloads can annotate workload `ServiceAccounts` with Managed Identity details necessary for creating clients. When the mutating admission webhook finds these annotations on a
`ServiceAccount` referenced by a pod being created, environment variables are set for the pod for the `AZURE_CLIENT_ID`, `AZURE_TENANT_ID` and `AZURE_FEDERATED_TOKEN_FILE`. The webhook also projects the service account token to the well-known path. Users should ensure that the `ServiceAccount` is annotated prior to creation of any pod requiring authentication or otherwise ensure that pods are
recreated afterwards.

This will be similar to how CCO deploys the [AWS Pod Identity webhook](https://github.com/openshift/aws-pod-identity-webhook) which we have forked for use by user workloads.

OpenShift's own ClusterOperators do not leverage the webhook, they are expected to natively support bound service account tokens.



#### Variation [optional]

TBD

### API Extensions

None as of now.

### Implementation Details/Notes/Constraints [optional]

TBD

### Risks and Mitigations

- The feature this work relies on was recently made public preview. What is the timeline for GA for Workload identity federation support for Managed Identities?
- How will security be reviewed and by whom?
- How will UX be reviewed and by whom?

### Drawbacks

The pod identity webhook deployed for AWS has received little ongoing maintenance since its initial deployment by CCO and this proposal adds yet another webhook to be lifecycled by CCO, however upstream seems to be moving in this direction for providing client details as opposed to config secrets. It is likely best for compatibility with how operators currently obtain client information from a
config secret while also respecting the environment variables that would be set by the webhook. Additionally, upstream projects may reject the notion of reading these details from a config secret but that has yet to be seen.

## Design Details

### Open Questions [optional]

### Test Plan

An e2e test job will be created similar to the [e2e-gcp-manual-oidc](https://github.com/openshift/release/pull/22552) that,
- Extracts `CredentialsRequests` from the release image.
- Processes `CredentialsRequests` with `ccoctl` to generate secret and `Authentication` configuration manifests.
- Moves the generated manifests into the manifests directory used for install.
- Runs the normal e2e suite against the resultant cluster.

### Graduation Criteria

#### Dev Preview -> Tech Preview

- Ability to utilize the enhancement end to end
- End user documentation, relative API stability
- Sufficient test coverage
- Gather feedback from users rather than just developers
- Enumerate service level indicators (SLIs), expose SLIs as metrics
- Write symptoms-based alerts for the component(s)

#### Tech Preview -> GA

Azure workload identity will be introduced as [TechPreviewNoUpgrade](https://github.com/openshift/api/blob/fefb3487546079495fb80ca0f1155ecd7417b9d8/config/v1/types_feature.go#L111) and then promoted once it is demonstrated to be working reliably.

- More testing (upgrade, downgrade, scale)
- Sufficient time for feedback
- Available by default
- User facing documentation created in [openshift-docs](https://github.com/openshift/openshift-docs/)

**For non-optional features moving to GA, the graduation criteria must include
end to end tests.**

#### Removing a deprecated feature

None.

### Upgrade / Downgrade Strategy

As clusters are upgraded, new permissions may be required or extended (in the case of future role granularity work) and users must evaluate those changes at the upgrade boundary similarly to [upgrading an STS cluster in manual mode](https://docs.openshift.com/container-platform/4.11/authentication/managing_cloud_provider_credentials/cco-mode-manual.html#manual-mode-sts-blurb).

### Version Skew Strategy

None.

### Operational Aspects of API Extensions

None.

#### Failure Modes

None.

#### Support Procedures

##### How to detect that operator credentials are incorrect / insufficient?

Operators will be degraded when credentials are insufficient / incorrect because operators will be unable to authenticate using the provided credentials or the permissions granted to the associated identity were insufficient. CCO will not monitor the state of the credentials on-cluster because CCO will be disabled based on clusters operating in `manual` credentials mode.

##### How to detect that the mutating webhook is degraded?

CCO will be degraded when unable to deploy the Azure pod identity mutating webhook (similar to the [AWS pod identity webhook controller](https://github.com/openshift/cloud-credential-operator/blob/4fb2c25c6f169e0b3e363b552b20603153e961d8/pkg/operator/awspodidentity/awspodidentitywebhook_controller.go#L254)) but will not monitor the health of the deployment.

Additionally,
- Webhook will set `failurePolicy=Ignore` and will not block pod creation when degraded.
- Webhook should be deployed with replicas >= 2 and a PDB to ensure that the webhook deployment is highly available.

## Implementation History

## Alternatives

## Infrastructure Needed [optional]

