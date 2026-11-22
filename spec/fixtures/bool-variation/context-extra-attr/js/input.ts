import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function betaBanner(userKey: string, email: string): Promise<boolean> {
  return await ldClient.boolVariation("beta-banner", { kind: "user", key: userKey, email }, false);
}
