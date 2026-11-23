import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();
const billingClient = OpenFeature.getClient("billing");

export async function checkout(userKey: string): Promise<boolean> {
  const on = await ldClient.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
  return on && client !== billingClient;
}
