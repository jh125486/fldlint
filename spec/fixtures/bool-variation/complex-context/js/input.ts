import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function orgFeature(userKey: string, orgKey: string): Promise<boolean> {
  const context = {
    kind: "multi" as const,
    user: { key: userKey },
    organization: { key: orgKey },
  };
  return await ldClient.boolVariation("org-feature", context, false);
}
