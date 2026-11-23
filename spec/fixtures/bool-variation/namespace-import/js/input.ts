import * as ld from "@launchdarkly/node-server-sdk";
import * as of from "@openfeature/server-sdk";

const ldClient = ld.init("sdk-key");
const client = of.OpenFeature.getClient();

export async function checkoutEnabled(userKey: string): Promise<boolean> {
  return await ldClient.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
}
