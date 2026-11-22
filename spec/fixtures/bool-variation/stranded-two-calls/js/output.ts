import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const client = OpenFeature.getClient();

export async function flags(userKey: string): Promise<[boolean, boolean]> {
  const ld = init("sdk-key");
  const checkout = await client.getBooleanValue("new-checkout", false, { targetingKey: userKey });
  const search = await client.getBooleanValue("new-search", false, { targetingKey: userKey });
  return [checkout, search];
}
