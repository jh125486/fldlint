import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function orgBanner(orgKey: string): Promise<boolean> {
  return await ldClient.boolVariation("org-banner", { kind: "organization", key: orgKey }, false);
}
