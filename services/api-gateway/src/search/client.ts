import { Client } from '@elastic/elasticsearch';
import { loadConfig } from '../config';

const config = loadConfig();

/**
 * Elasticsearch client connected to the configured cluster.
 */
export const elasticsearchClient = new Client({
  node: config.ELASTICSEARCH_URL,
});

export interface SearchResult {
  productId: string;
  name: string;
  description: string;
  price: number;
  score: number;
}

interface ProductSource {
  productId?: string;
  product_id?: string;
  name: string;
  description: string;
  price: number;
}

/**
 * Searches the "products" index using a multi-match query across name and description fields.
 */
export async function searchProducts(query: string): Promise<SearchResult[]> {
  const response = await elasticsearchClient.search<ProductSource>({
    index: 'products',
    body: {
      query: {
        multi_match: {
          query,
          fields: ['name^2', 'description'],
          type: 'best_fields',
          fuzziness: 'AUTO',
        },
      },
      size: 20,
    },
  });

  return response.hits.hits.map((hit) => ({
    productId: hit._source?.productId ?? hit._source?.product_id ?? hit._id,
    name: hit._source?.name ?? '',
    description: hit._source?.description ?? '',
    price: hit._source?.price ?? 0,
    score: hit._score ?? 0,
  }));
}
