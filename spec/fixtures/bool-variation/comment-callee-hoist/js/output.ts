import { init } from '@launchdarkly/node-server-sdk';
import { OpenFeature } from '@openfeature/server-sdk';

const ldClient = init('sdk-key');
const client = OpenFeature.getClient();

export async function banner(userKey: string): Promise<boolean> {
  return await /* a */ /* b */ /* p */ client.getBooleanValue('promo', false, { targetingKey: userKey });
}
