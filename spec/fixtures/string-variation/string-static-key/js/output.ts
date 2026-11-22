import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function checkoutLayout(userKey: string): Promise<string> {
  return await client.getStringValue("checkout-layout", "classic", { targetingKey: userKey });
}
