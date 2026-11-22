import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function menuLabel(userKey: string): Promise<string> {
  const label = "café 😀", enabled = await ldClient.boolVariation("cafe-menu", { kind: "user", key: userKey }, false);
  return enabled ? label : "";
}
