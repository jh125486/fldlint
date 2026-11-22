import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const client = OpenFeature.getClient();

export async function checkoutEnabled(userKey: string): Promise<boolean> {
  const ld = init("sdk-key");
  return await client.getBooleanValue("new-checkout", false, { targetingKey: userKey });
}
