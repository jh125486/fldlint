import { init } from "launchdarkly-node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function whyEnabled(userKey: string): Promise<string> {
  const detail = await ldClient.boolVariationDetail("new-checkout", { kind: "user", key: userKey }, false);
  return detail.reason.kind;
}
