import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");

export function billing(): string {
  const billingClient = OpenFeature.getClient("billing");
  return billingClient.metadata.name ?? "";
}

export async function checkout(userKey: string): Promise<boolean> {
  return await ldClient.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
}
