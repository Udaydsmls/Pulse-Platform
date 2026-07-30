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

const SECRET_PREFIX = 'pulse-platform/prod';

/**
 * Environment variable name -> Secrets Manager secret name.
 *
 * The keys must match the variable names the service actually reads, and the
 * ones the Helm values reference with $(VAR) — Kubernetes expands those from
 * other variables on the same container, all of which land here.
 */
type SecretMap = Record<string, string>;

// Every service that talks to Postgres or Kafka needs these.
const DATABASE: SecretMap = {
  DB_HOST: `${SECRET_PREFIX}/rds-host`,
  DB_PASSWORD: `${SECRET_PREFIX}/rds-password`,
};

const KAFKA: SecretMap = {
  KAFKA_BOOTSTRAP_SERVERS: `${SECRET_PREFIX}/msk-bootstrap-servers`,
};

const SERVICE_SECRETS: Record<ServiceName, SecretMap> = {
  'user-service': {
    ...DATABASE,
    ...KAFKA,
    JWT_SECRET: `${SECRET_PREFIX}/jwt-secret`,
  },

  'order-service': { ...DATABASE, ...KAFKA },
  'inventory-service': { ...DATABASE, ...KAFKA },
  'payment-service': { ...DATABASE, ...KAFKA },

  // DynamoDB access comes from the pod's IAM role, not a secret, so this
  // service only needs Kafka.
  'notification-service': { ...KAFKA },

  // The gateway has no database. It shares the JWT secret with user-service so
  // it can verify the tokens user-service signs.
  'api-gateway': {
    ...KAFKA,
    JWT_SECRET: `${SECRET_PREFIX}/jwt-secret`,
    REDIS_URL: `${SECRET_PREFIX}/elasticache-url`,
  },
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
      // secretKey becomes the key in the Kubernetes secret, which envFrom turns
      // into an environment variable of the same name.
      const remoteRefs = Object.entries(SERVICE_SECRETS[service]).map(
        ([envVar, secretName]) => ({
          secretKey: envVar,
          remoteRef: { key: secretName },
        }),
      );

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
