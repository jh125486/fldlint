import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function menuLabel(userKey: string): Promise<string> {
  const label = "café 😀", enabled = await client.getBooleanValue("cafe-menu", false, { targetingKey: userKey });
  return enabled ? label : "";
}
