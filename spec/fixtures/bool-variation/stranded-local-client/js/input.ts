import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const client = OpenFeature.getClient();

export async function checkoutEnabled(userKey: string): Promise<boolean> {
  const ld = init("sdk-key");
  return await ld.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
}
