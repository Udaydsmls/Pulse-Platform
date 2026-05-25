import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import { Construct } from 'constructs';

export interface AppStackProps extends cdk.StackProps {
  readonly eksClusterName: string;
  readonly eksClusterOidcProvider: iam.IOpenIdConnectProvider;
}

const SERVICES = [
  'user-service',
  'order-service',
  'inventory-service',
  'payment-service',
  'notification-service',
  'api-gateway',
] as const;

type ServiceName = typeof SERVICES[number];

/** Maps each service to the Secrets Manager secret names it needs. */
const SERVICE_SECRETS: Record<ServiceName, string[]> = {
  'user-service':         ['pulse-platform/prod/rds-password', 'pulse-platform/prod/oauth-secret'],
  'order-service':        ['pulse-platform/prod/rds-password'],
  'inventory-service':    ['pulse-platform/prod/rds-password'],
  'payment-service':      ['pulse-platform/prod/rds-password', 'pulse-platform/prod/stripe-secret'],
  'notification-service': ['pulse-platform/prod/rds-password', 'pulse-platform/prod/smtp-secret'],
  'api-gateway':          ['pulse-platform/prod/oauth-secret', 'pulse-platform/prod/jwt-secret'],
};

export class AppStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: AppStackProps) {
    super(scope, id, props);

    const { eksClusterName, eksClusterOidcProvider } = props;

    const kubectlRoleArn: string = this.node.tryGetContext('eksKubectlRoleArn') ?? '';
    const clusterEndpoint: string = this.node.tryGetContext('eksClusterEndpoint') ?? '';
    const clusterCertAuthority: string = this.node.tryGetContext('eksClusterCertAuthority') ?? '';

    const cluster = eks.Cluster.fromClusterAttributes(this, 'ImportedCluster', {
      clusterName: eksClusterName,
      kubectlRoleArn,
      clusterEndpoint,
      clusterCertificateAuthorityData: clusterCertAuthority,
      openIdConnectProvider: eksClusterOidcProvider,
    });

    // ── External Secrets Operator: ClusterSecretStore ─────────────────────────

    const secretsReadRoleArn = cdk.Fn.importValue('PulsePlatform-SecretsReadRoleArn');

    cluster.addManifest('ClusterSecretStore', {
      apiVersion: 'external-secrets.io/v1beta1',
      kind: 'ClusterSecretStore',
      metadata: {
        name: 'aws-secrets-manager',
        annotations: {
          'app.kubernetes.io/managed-by': 'aws-cdk',
        },
      },
      spec: {
        provider: {
          aws: {
            service: 'SecretsManager',
            region: this.region,
            auth: {
              jwt: {
                serviceAccountRef: {
                  name: 'external-secrets-sa',
                  namespace: 'external-secrets',
                },
              },
            },
          },
        },
      },
    });

    // ── ExternalSecret per service ────────────────────────────────────────────

    SERVICES.forEach((service) => {
      const secrets = SERVICE_SECRETS[service];

      const remoteRefs = secrets.map((secretName) => ({
        secretKey: secretName.split('/').pop()!, // use last segment as key
        remoteRef: {
          key: secretName,
        },
      }));

      cluster.addManifest(`ExternalSecret-${service}`, {
        apiVersion: 'external-secrets.io/v1beta1',
        kind: 'ExternalSecret',
        metadata: {
          name: `${service}-secrets`,
          namespace: 'pulse-platform',
        },
        spec: {
          refreshInterval: '1h',
          secretStoreRef: {
            name: 'aws-secrets-manager',
            kind: 'ClusterSecretStore',
          },
          target: {
            name: `${service}-secrets`,
            creationPolicy: 'Owner',
          },
          data: remoteRefs,
        },
      });
    });

    // ── ECR Lifecycle Policies ─────────────────────────────────────────────────

    SERVICES.forEach((service) => {
      const repo = ecr.Repository.fromRepositoryName(
        this,
        `EcrRepo-${service}`,
        `pulse-platform/${service}`,
      );

      // CDK's addLifecycleRule supports keeping only the last N images.
      (repo as ecr.Repository).addLifecycleRule?.({
        description: 'Keep last 10 tagged images',
        maxImageCount: 10,
        tagStatus: ecr.TagStatus.TAGGED,
        tagPrefixList: ['v'],
      });

      (repo as ecr.Repository).addLifecycleRule?.({
        description: 'Expire untagged images after 7 days',
        maxImageAge: cdk.Duration.days(7),
        tagStatus: ecr.TagStatus.UNTAGGED,
      });
    });

    // ── Outputs ───────────────────────────────────────────────────────────────

    new cdk.CfnOutput(this, 'ClusterSecretStoreName', {
      value: 'aws-secrets-manager',
      description: 'Name of the ClusterSecretStore for External Secrets Operator',
    });
  }
}
