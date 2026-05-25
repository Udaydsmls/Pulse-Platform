import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as iam from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';

export interface EksStackProps extends cdk.StackProps {}

export class EksStack extends cdk.Stack {
  /** The name of the imported EKS cluster. */
  public readonly clusterName: string;
  /** OIDC provider ARN — used by AppStack for IRSA. */
  public readonly clusterOidcProvider: iam.IOpenIdConnectProvider;

  constructor(scope: Construct, id: string, props: EksStackProps) {
    super(scope, id, props);

    // ── Import cluster provisioned by Terraform ──────────────────────────────
    // The cluster name and kubectl role ARN are resolved from SSM or passed
    // as context values so this CDK app can operate without re-creating the
    // cluster.
    const clusterName = this.node.tryGetContext('eksClusterName') ?? 'pulse-platform';
    const kubectlRoleArn: string = this.node.tryGetContext('eksKubectlRoleArn') ?? '';
    const clusterEndpoint: string = this.node.tryGetContext('eksClusterEndpoint') ?? '';
    const clusterCertAuthority: string = this.node.tryGetContext('eksClusterCertAuthority') ?? '';
    const openIdConnectProviderArn: string = this.node.tryGetContext('eksOidcProviderArn') ?? '';

    this.clusterName = clusterName;

    const cluster = eks.Cluster.fromClusterAttributes(this, 'ImportedCluster', {
      clusterName,
      kubectlRoleArn,
      clusterEndpoint,
      clusterCertificateAuthorityData: clusterCertAuthority,
      openIdConnectProvider: iam.OpenIdConnectProvider.fromOpenIdConnectProviderArn(
        this,
        'OidcProvider',
        openIdConnectProviderArn,
      ),
    });

    this.clusterOidcProvider = iam.OpenIdConnectProvider.fromOpenIdConnectProviderArn(
      this,
      'OidcProviderRef',
      openIdConnectProviderArn,
    );

    // ── Additional Node Groups ────────────────────────────────────────────────

    // On-demand node group (imported via L1 because we use fromClusterAttributes)
    new eks.CfnNodegroup(this, 'OnDemandNodeGroup', {
      clusterName,
      nodegroupName: 'pulse-on-demand',
      nodeRole: this.onDemandNodeRoleArn(clusterName),
      subnets: (this.node.tryGetContext('privateSubnetIds') ?? '').split(','),
      instanceTypes: ['t3.large'],
      scalingConfig: { minSize: 2, maxSize: 6, desiredSize: 3 },
      capacityType: 'ON_DEMAND',
      labels: { 'node-type': 'on-demand' },
      tags: { Project: 'pulse-platform', Environment: 'prod' },
    });

    new eks.CfnNodegroup(this, 'SpotNodeGroup', {
      clusterName,
      nodegroupName: 'pulse-spot',
      nodeRole: this.onDemandNodeRoleArn(clusterName),
      subnets: (this.node.tryGetContext('privateSubnetIds') ?? '').split(','),
      instanceTypes: ['t3.large', 't3.xlarge', 'm5.large'],
      scalingConfig: { minSize: 0, maxSize: 10, desiredSize: 2 },
      capacityType: 'SPOT',
      labels: { 'node-type': 'spot' },
      taints: [{ effect: 'NO_SCHEDULE', key: 'spot', value: 'true' }],
      tags: { Project: 'pulse-platform', Environment: 'prod' },
    });

    // ── Kubernetes Namespaces ─────────────────────────────────────────────────

    const namespaces = [
      'pulse-platform',
      'istio-system',
      'monitoring',
      'external-secrets',
    ];

    namespaces.forEach((ns) => {
      cluster.addManifest(`Namespace-${ns}`, {
        apiVersion: 'v1',
        kind: 'Namespace',
        metadata: {
          name: ns,
          labels: {
            'app.kubernetes.io/managed-by': 'aws-cdk',
            'project': 'pulse-platform',
          },
        },
      });
    });

    // ── IRSA: S3 Read ─────────────────────────────────────────────────────────

    const s3ReadRole = new iam.Role(this, 'S3ReadIrsaRole', {
      roleName: 'pulse-platform-s3-read',
      assumedBy: new iam.WebIdentityPrincipal(openIdConnectProviderArn, {
        StringEquals: {
          [`${openIdConnectProviderArn.replace('arn:aws:iam::' + this.account + ':oidc-provider/', '')}:sub`]:
            'system:serviceaccount:pulse-platform:pulse-s3-reader',
        },
      }),
      description: 'IRSA role granting S3 read access to pulse-platform pods',
    });

    s3ReadRole.addManagedPolicy(iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonS3ReadOnlyAccess'));

    // ── IRSA: Secrets Manager Read ────────────────────────────────────────────

    const secretsReadRole = new iam.Role(this, 'SecretsManagerIrsaRole', {
      roleName: 'pulse-platform-secrets-read',
      assumedBy: new iam.WebIdentityPrincipal(openIdConnectProviderArn, {
        StringEquals: {
          [`${openIdConnectProviderArn.replace('arn:aws:iam::' + this.account + ':oidc-provider/', '')}:sub`]:
            'system:serviceaccount:pulse-platform:external-secrets-sa',
        },
      }),
      description: 'IRSA role for External Secrets Operator to read AWS Secrets Manager',
    });

    secretsReadRole.addToPolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: [
          'secretsmanager:GetSecretValue',
          'secretsmanager:DescribeSecret',
          'secretsmanager:ListSecretVersionIds',
        ],
        resources: [`arn:aws:secretsmanager:${this.region}:${this.account}:secret:pulse-platform/*`],
      }),
    );

    // ── IRSA: DynamoDB Write for notification-service ─────────────────────────

    const dynamoWriteRole = new iam.Role(this, 'DynamoWriteIrsaRole', {
      roleName: 'pulse-platform-dynamo-write',
      assumedBy: new iam.WebIdentityPrincipal(openIdConnectProviderArn, {
        StringEquals: {
          [`${openIdConnectProviderArn.replace('arn:aws:iam::' + this.account + ':oidc-provider/', '')}:sub`]:
            'system:serviceaccount:pulse-platform:notification-service',
        },
      }),
      description: 'IRSA role granting DynamoDB write access to notification-service',
    });

    dynamoWriteRole.addToPolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: [
          'dynamodb:PutItem',
          'dynamodb:UpdateItem',
          'dynamodb:DeleteItem',
          'dynamodb:BatchWriteItem',
          'dynamodb:GetItem',
          'dynamodb:Query',
          'dynamodb:Scan',
        ],
        resources: [
          `arn:aws:dynamodb:${this.region}:${this.account}:table/pulse-platform-*`,
        ],
      }),
    );

    // ── SSM Outputs (consumed by AppStack) ────────────────────────────────────

    new cdk.CfnOutput(this, 'ClusterName', {
      value: clusterName,
      exportName: 'PulsePlatform-EksClusterName',
    });

    new cdk.CfnOutput(this, 'SecretsReadRoleArn', {
      value: secretsReadRole.roleArn,
      exportName: 'PulsePlatform-SecretsReadRoleArn',
    });
  }

  /** Resolves the node IAM role ARN for CfnNodegroup. In a real deployment
   *  this would be retrieved from SSM/context, here we construct the expected
   *  name convention from the cluster name. */
  private onDemandNodeRoleArn(clusterName: string): string {
    return this.node.tryGetContext('eksNodeRoleArn')
      ?? `arn:aws:iam::${this.account}:role/${clusterName}-eks-node-role`;
  }
}
