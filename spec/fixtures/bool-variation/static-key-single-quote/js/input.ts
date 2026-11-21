import { init } from 'launchdarkly-node-server-sdk';
import { OpenFeature } from '@openfeature/server-sdk';

const ldClient = init('sdk-key');
const client = OpenFeature.getClient();

export async function betaBanner(userKey: string): Promise<boolean> {
  const context = { kind: 'user', key: userKey };
  return await ldClient.boolVariation('beta-banner', context, false);
}
