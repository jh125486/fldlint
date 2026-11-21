import { init } from "launchdarkly-node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function checkoutEnabled(userKey: string): Promise<boolean> {
  const context = { kind: "user", key: userKey };
  const enabled = await ldClient.boolVariation("new-checkout", context, false);
  return enabled;
}
