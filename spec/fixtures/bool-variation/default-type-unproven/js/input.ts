import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function featureOn(userKey: string, fallback: unknown): Promise<boolean> {
  return await ldClient.boolVariation("new-feature", { kind: "user", key: userKey }, fallback);
}
