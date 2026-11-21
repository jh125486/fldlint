import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function checkoutLayout(userKey: string): Promise<string> {
  return await ldClient.stringVariation("checkout-layout", { kind: "user", key: userKey }, "classic");
}
